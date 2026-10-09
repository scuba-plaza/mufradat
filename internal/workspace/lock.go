package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"
)

const lockPoll = 25 * time.Millisecond

func (w Workspace) Lock(ctx context.Context) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(w.LockFile()), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(w.LockFile(), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	ticker := time.NewTicker(lockPoll)
	defer ticker.Stop()
	for {
		err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			return func() {
				_ = unix.Flock(int(f.Fd()), unix.LOCK_UN)
				_ = f.Close()
			}, nil
		}
		if !errors.Is(err, unix.EWOULDBLOCK) && !errors.Is(err, unix.EINTR) {
			_ = f.Close()
			return nil, fmt.Errorf("lock %s: %w", w.LockFile(), err)
		}
		select {
		case <-ctx.Done():
			_ = f.Close()
			return nil, fmt.Errorf("waiting for workspace lock: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}
