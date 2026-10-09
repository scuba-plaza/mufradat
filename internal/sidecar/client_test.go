package sidecar

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/scuba-plaza/mufradat/internal/proc"
	"github.com/scuba-plaza/mufradat/internal/verify"
)

type message map[string]any

type behaviour func(req request, send func(message), stopped <-chan struct{}) (exit bool)

type fakeProcess struct {
	inR     *io.PipeReader
	inW     *io.PipeWriter
	outR    *io.PipeReader
	outW    *io.PipeWriter
	stopped chan struct{}
	exited  chan struct{}
	once    sync.Once

	mu       sync.Mutex
	requests []request
}

func startFake(hello message, run behaviour) *fakeProcess {
	p := &fakeProcess{stopped: make(chan struct{}), exited: make(chan struct{})}
	p.inR, p.inW = io.Pipe()
	p.outR, p.outW = io.Pipe()
	go func() {
		defer close(p.exited)
		defer p.outW.Close()
		send := func(m message) {
			line, _ := json.Marshal(m)
			_, _ = p.outW.Write(append(line, '\n'))
		}
		if hello != nil {
			send(hello)
		}
		r := bufio.NewReader(p.inR)
		for {
			line, err := r.ReadBytes('\n')
			if len(bytes.TrimSpace(line)) > 0 {
				var req request
				if jerr := json.Unmarshal(line, &req); jerr == nil {
					p.mu.Lock()
					p.requests = append(p.requests, req)
					p.mu.Unlock()
					if req.Type == "shutdown" || run(req, send, p.stopped) {
						return
					}
				}
			}
			if err != nil {
				return
			}
		}
	}()
	return p
}

func (p *fakeProcess) Stdin() io.WriteCloser { return p.inW }
func (p *fakeProcess) Stdout() io.ReadCloser { return p.outR }
func (p *fakeProcess) Wait() error           { <-p.exited; return nil }
func (p *fakeProcess) Stop() {
	p.once.Do(func() {
		close(p.stopped)
		_ = p.inR.CloseWithError(io.ErrClosedPipe)
		_ = p.outW.CloseWithError(io.ErrClosedPipe)
	})
}

func (p *fakeProcess) seen() []request {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.requests)
}

var goodHello = message{"type": "hello", "protocol": 1, "version": "1", "versions": message{"python": "3.13.1", "torch": nil}}

func launcherFor(p *fakeProcess) Launcher {
	return func(context.Context, io.Writer) (proc.Process, error) { return p, nil }
}

func answerAll(req request, send func(message), _ <-chan struct{}) bool {
	if req.Type != "check" {
		send(message{"type": "error", "id": req.ID, "message": "unknown request"})
		return false
	}
	send(message{"type": "status", "id": req.ID, "message": "loading"})
	for i, it := range req.Items {
		if it.Tokens[0] == "bad" {
			send(message{"type": "item_error", "id": req.ID, "key": it.Key, "message": "KeyError: bad"})
		} else {
			send(message{"type": "reading", "id": req.ID, "key": it.Key, "analyses": [][]string{{"كِتَاب"}}, "catt": nil, "bert": []string{"كِتَاب"}})
		}
		send(message{"type": "progress", "id": req.ID, "done": i + 1, "total": len(req.Items)})
	}
	send(message{"type": "done", "id": req.ID})
	return false
}

func items(words ...string) []verify.Item {
	out := make([]verify.Item, len(words))
	for i, w := range words {
		out[i] = verify.Item{Key: "k-" + w, Field: verify.FieldLemma, Tokens: []string{w}, Citation: true}
	}
	return out
}

type recorder struct {
	readings []verify.Reading
	failures map[string]string
	statuses []string
	last     [2]int
}

func (r *recorder) events() verify.Events {
	r.failures = map[string]string{}
	return verify.Events{
		Reading:  func(x verify.Reading) { r.readings = append(r.readings, x) },
		Failure:  func(k, m string) { r.failures[k] = m },
		Progress: func(d, t int) { r.last = [2]int{d, t} },
		Status:   func(m string) { r.statuses = append(r.statuses, m) },
	}
}

func TestCheckStreamsEvents(t *testing.T) {
	p := startFake(goodHello, answerAll)
	c, err := Start(context.Background(), launcherFor(p), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if c.Hello().Versions["python"] == nil || *c.Hello().Versions["python"] != "3.13.1" || c.Hello().Versions["torch"] != nil {
		t.Fatalf("hello = %+v", c.Hello())
	}
	var rec recorder
	if err := c.Check(context.Background(), items("a", "bad", "c"), rec.events()); err != nil {
		t.Fatal(err)
	}
	if len(rec.readings) != 2 || rec.readings[0].Key != "k-a" || rec.readings[1].BERT[0] != "كِتَاب" || rec.readings[0].CATT != nil {
		t.Fatalf("readings = %+v", rec.readings)
	}
	if rec.failures["k-bad"] != "KeyError: bad" || rec.last != [2]int{3, 3} || !slices.Equal(rec.statuses, []string{"loading"}) {
		t.Fatalf("failures=%v last=%v statuses=%v", rec.failures, rec.last, rec.statuses)
	}
}

func TestSecondCheckReusesTheProcess(t *testing.T) {
	p := startFake(goodHello, answerAll)
	c, err := Start(context.Background(), launcherFor(p), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	var first, second recorder
	if err := c.Check(context.Background(), items("a"), first.events()); err != nil {
		t.Fatal(err)
	}
	if err := c.Check(context.Background(), items("b", "c"), second.events()); err != nil {
		t.Fatal(err)
	}
	if len(first.readings) != 1 || len(second.readings) != 2 {
		t.Fatalf("first=%d second=%d", len(first.readings), len(second.readings))
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	reqs := p.seen()
	if len(reqs) != 3 || reqs[0].ID == reqs[1].ID || reqs[2].Type != "shutdown" {
		t.Fatalf("requests = %+v", reqs)
	}
}

func TestEmptyCheckSendsNothing(t *testing.T) {
	p := startFake(goodHello, answerAll)
	c, _ := Start(context.Background(), launcherFor(p), io.Discard)
	defer c.Close()
	if err := c.Check(context.Background(), nil, verify.Events{}); err != nil {
		t.Fatal(err)
	}
	if len(p.seen()) != 0 {
		t.Fatalf("requests = %+v", p.seen())
	}
}

func TestWrongProtocolIsRejected(t *testing.T) {
	p := startFake(message{"type": "hello", "protocol": 9, "version": "x", "versions": message{}}, answerAll)
	_, err := Start(context.Background(), launcherFor(p), io.Discard)
	if err == nil || !strings.Contains(err.Error(), "protocol 9") {
		t.Fatalf("err = %v", err)
	}
}

func TestStartExplainsAProcessThatPrintsGarbage(t *testing.T) {
	p := startFake(message{"type": "oops"}, answerAll)
	_, err := Start(context.Background(), launcherFor(p), io.Discard)
	if err == nil || !strings.Contains(err.Error(), "unexpected") {
		t.Fatalf("err = %v", err)
	}
}

func TestStartReportsALaunchFailure(t *testing.T) {
	want := errors.New("python: not found")
	_, err := Start(context.Background(), func(context.Context, io.Writer) (proc.Process, error) { return nil, want }, io.Discard)
	if !errors.Is(err, want) {
		t.Fatalf("err = %v", err)
	}
}

func TestFatalErrorFromTheSidecar(t *testing.T) {
	p := startFake(goodHello, func(req request, send func(message), _ <-chan struct{}) bool {
		send(message{"type": "error", "id": req.ID, "message": "OSError: models missing"})
		return false
	})
	c, _ := Start(context.Background(), launcherFor(p), io.Discard)
	defer c.Close()
	err := c.Check(context.Background(), items("a"), verify.Events{})
	if err == nil || !strings.Contains(err.Error(), "OSError: models missing") {
		t.Fatalf("err = %v", err)
	}
	if again := c.Check(context.Background(), items("a"), verify.Events{}); again == nil {
		t.Fatal("a failed client should stay failed")
	}
}

func TestCrashMidCheckKeepsEarlierReadings(t *testing.T) {
	p := startFake(goodHello, func(req request, send func(message), _ <-chan struct{}) bool {
		send(message{"type": "reading", "id": req.ID, "key": req.Items[0].Key, "analyses": [][]string{{"x"}}})
		return true
	})
	c, _ := Start(context.Background(), launcherFor(p), io.Discard)
	defer c.Close()
	var rec recorder
	err := c.Check(context.Background(), items("a", "b"), rec.events())
	if err == nil || !strings.Contains(err.Error(), "stopped unexpectedly") {
		t.Fatalf("err = %v", err)
	}
	if len(rec.readings) != 1 {
		t.Fatalf("readings = %+v", rec.readings)
	}
}

func TestCancelStopsTheProcess(t *testing.T) {
	started := make(chan struct{})
	p := startFake(goodHello, func(req request, send func(message), stopped <-chan struct{}) bool {
		send(message{"type": "status", "id": req.ID, "message": "working"})
		close(started)
		<-stopped
		return true
	})
	c, _ := Start(context.Background(), launcherFor(p), io.Discard)
	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() { errc <- c.Check(ctx, items("a"), verify.Events{}) }()
	<-started
	cancel()
	select {
	case err := <-errc:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Check did not return after cancel")
	}
	select {
	case <-p.stopped:
	default:
		t.Fatal("the process was not stopped")
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestIgnoresEventsFromOtherRequests(t *testing.T) {
	p := startFake(goodHello, func(req request, send func(message), _ <-chan struct{}) bool {
		send(message{"type": "reading", "id": req.ID + 7, "key": "stray"})
		send(message{"type": "done", "id": req.ID})
		return false
	})
	c, _ := Start(context.Background(), launcherFor(p), io.Discard)
	defer c.Close()
	var rec recorder
	if err := c.Check(context.Background(), items("a"), rec.events()); err != nil || len(rec.readings) != 0 {
		t.Fatalf("err=%v readings=%+v", err, rec.readings)
	}
}

func TestLogWriterSplitsLinesAndCarriageReturns(t *testing.T) {
	var out bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&out, &slog.HandlerOptions{Level: slog.LevelDebug, ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
		if a.Key == slog.TimeKey {
			return slog.Attr{}
		}
		return a
	}}))
	w := LogWriter(logger)
	_, _ = w.Write([]byte("loading 10%\rloading 50%\rdone\npartial"))
	_, _ = w.Write([]byte(" line\n\n"))
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 4 || !strings.Contains(lines[0], "loading 10%") || !strings.Contains(lines[3], "partial line") {
		t.Fatalf("log output:\n%s", out.String())
	}
}
