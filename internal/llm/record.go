package llm

import (
	"context"
	"time"
)

type Recording struct {
	Next Backend
	Now  func() time.Time
	Sink func(Record) error
}

func (r Recording) Complete(ctx context.Context, req Request) (Response, error) {
	resp, err := r.Next.Complete(ctx, req)
	if err == nil && r.Sink != nil {
		if serr := r.Sink(Record{Time: r.Now().UTC().Truncate(time.Second), Usage: resp.Usage}); serr != nil {
			return resp, serr
		}
	}
	return resp, err
}
