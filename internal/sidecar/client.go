package sidecar

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/scuba-plaza/mufradat/internal/proc"
	"github.com/scuba-plaza/mufradat/internal/verify"
)

const (
	Protocol     = 1
	helloTimeout = 2 * time.Minute
	closeGrace   = 5 * time.Second
)

type Launcher func(ctx context.Context, stderr io.Writer) (proc.Process, error)

type Hello struct {
	Protocol int                `json:"protocol"`
	Version  string             `json:"version"`
	Versions map[string]*string `json:"versions"`
}

type wire struct {
	Type     string     `json:"type"`
	ID       int        `json:"id"`
	Message  string     `json:"message"`
	Key      string     `json:"key"`
	Done     int        `json:"done"`
	Total    int        `json:"total"`
	Analyses [][]string `json:"analyses"`
	CATT     []string   `json:"catt"`
	BERT     []string   `json:"bert"`
	Hello
}

type request struct {
	Type  string        `json:"type"`
	ID    int           `json:"id,omitempty"`
	Items []verify.Item `json:"items,omitempty"`
}

type Client struct {
	process proc.Process
	hello   Hello
	events  chan wire
	stop    chan struct{}
	stopped sync.Once
	done    chan struct{}
	exited  chan struct{}

	mu     sync.Mutex
	nextID int
	dead   error
	closed bool
}

func Start(ctx context.Context, launch Launcher, stderr io.Writer) (*Client, error) {
	p, err := launch(ctx, stderr)
	if err != nil {
		return nil, err
	}
	c := &Client{process: p, events: make(chan wire), stop: make(chan struct{}), done: make(chan struct{}), exited: make(chan struct{})}
	go c.read(bufio.NewReaderSize(p.Stdout(), 1<<20))
	go c.reap()

	timer := time.NewTimer(helloTimeout)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		c.kill()
		return nil, ctx.Err()
	case <-timer.C:
		c.kill()
		return nil, errors.New("the verification process did not introduce itself in time")
	case e := <-c.events:
		if e.Type != "hello" {
			c.kill()
			return nil, c.explain(e)
		}
		if e.Protocol != Protocol {
			c.kill()
			return nil, fmt.Errorf("the verification process speaks protocol %d, this program speaks %d", e.Protocol, Protocol)
		}
		c.hello = e.Hello
	}
	return c, nil
}

func (c *Client) Hello() Hello { return c.hello }

func (c *Client) reap() {
	<-c.done
	_ = c.process.Wait()
	close(c.exited)
}

func (c *Client) read(r *bufio.Reader) {
	defer close(c.done)
	defer close(c.events)
	for {
		line, err := r.ReadBytes('\n')
		if text := strings.TrimSpace(string(line)); text != "" {
			var w wire
			if jerr := json.Unmarshal([]byte(text), &w); jerr != nil {
				w = wire{Type: "garbage", Message: text}
			}
			select {
			case c.events <- w:
			case <-c.stop:
				return
			}
		}
		if err != nil {
			return
		}
	}
}

func (c *Client) explain(e wire) error {
	switch e.Type {
	case "error":
		return fmt.Errorf("the verification process failed: %s", e.Message)
	case "garbage":
		return fmt.Errorf("the verification process printed something unexpected: %s", truncate(e.Message, 200))
	}
	return fmt.Errorf("the verification process sent an unexpected %q message", e.Type)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func (c *Client) kill() {
	c.stopped.Do(func() {
		close(c.stop)
		c.process.Stop()
	})
}

func (c *Client) Check(ctx context.Context, items []verify.Item, ev verify.Events) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return errors.New("the verification process is closed")
	}
	if c.dead != nil {
		return c.dead
	}
	if len(items) == 0 {
		return nil
	}
	c.nextID++
	id := c.nextID
	line, err := json.Marshal(request{Type: "check", ID: id, Items: items})
	if err != nil {
		return err
	}
	if _, err := c.process.Stdin().Write(append(line, '\n')); err != nil {
		c.dead = fmt.Errorf("the verification process is not accepting work: %w", err)
		return c.dead
	}
	for {
		select {
		case <-ctx.Done():
			c.dead = ctx.Err()
			c.kill()
			return ctx.Err()
		case e, ok := <-c.events:
			if !ok {
				c.dead = errors.New("the verification process stopped unexpectedly")
				return c.dead
			}
			finished, err := c.dispatch(id, e, ev)
			if err != nil {
				c.dead = err
				c.kill()
				return err
			}
			if finished {
				return nil
			}
		}
	}
}

func (c *Client) dispatch(id int, e wire, ev verify.Events) (bool, error) {
	if e.ID != 0 && e.ID != id {
		return false, nil
	}
	switch e.Type {
	case "status":
		if ev.Status != nil {
			ev.Status(e.Message)
		}
	case "progress":
		if ev.Progress != nil {
			ev.Progress(e.Done, e.Total)
		}
	case "reading":
		if ev.Reading != nil {
			ev.Reading(verify.Reading{Key: e.Key, Analyses: e.Analyses, CATT: e.CATT, BERT: e.BERT})
		}
	case "item_error":
		if ev.Failure != nil {
			ev.Failure(e.Key, e.Message)
		}
	case "done":
		return true, nil
	default:
		return false, c.explain(e)
	}
	return false, nil
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	if c.dead == nil {
		if line, err := json.Marshal(request{Type: "shutdown"}); err == nil {
			_, _ = c.process.Stdin().Write(append(line, '\n'))
		}
	}
	_ = c.process.Stdin().Close()
	select {
	case <-c.exited:
	case <-time.After(closeGrace):
		c.kill()
		<-c.exited
	}
	c.kill()
	return nil
}

type logWriter struct {
	logger *slog.Logger
	mu     sync.Mutex
	buf    []byte
}

func LogWriter(logger *slog.Logger) io.Writer { return &logWriter{logger: logger} }

func (w *logWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf = append(w.buf, p...)
	for {
		i := strings.IndexAny(string(w.buf), "\r\n")
		if i < 0 {
			break
		}
		if text := strings.TrimSpace(string(w.buf[:i])); text != "" {
			w.logger.Debug(text, "source", "sidecar")
		}
		w.buf = w.buf[i+1:]
	}
	return len(p), nil
}
