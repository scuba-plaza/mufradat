package pyenv

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/scuba-plaza/mufradat/internal/failure"
	"github.com/scuba-plaza/mufradat/internal/fsutil"
	"github.com/scuba-plaza/mufradat/internal/proc"
)

const setupHint = "run `mufradat setup` to install the vowel checkers, or set verify.enabled = \"off\""

type Runtime struct {
	Python      string
	Fingerprint string
	Managed     bool
}

func (m *Manager) Resolve(custom string) (Runtime, error) {
	src, err := m.SourceDigest()
	if err != nil {
		return Runtime{}, err
	}
	if custom != "" {
		py, err := m.Runner.LookPath(custom)
		if err != nil {
			return Runtime{}, failure.NotSetUp("check verify.python, or remove it to use the managed environment",
				fmt.Errorf("verify.python %q was not found: %w", custom, err))
		}
		return Runtime{Python: py, Fingerprint: CustomFingerprint(py, src)}, nil
	}
	ready, ok, err := m.ReadyState()
	if err != nil {
		return Runtime{}, err
	}
	if !ok {
		return Runtime{}, failure.NotSetUp(setupHint, errors.New("the vowel-checking environment is not set up"))
	}
	py, err := m.Interpreter()
	if err != nil {
		return Runtime{}, err
	}
	if !fsutil.Exists(py) {
		return Runtime{}, failure.NotSetUp("run `mufradat setup --force` to rebuild it",
			fmt.Errorf("the vowel-checking environment is damaged: %s is missing", py))
	}
	return Runtime{Python: py, Fingerprint: fingerprint(ready.LockDigest, src, ready.CamelPackages), Managed: true}, nil
}

func (m *Manager) Launcher(rt Runtime) func(ctx context.Context, stderr io.Writer) (proc.Process, error) {
	return func(ctx context.Context, stderr io.Writer) (proc.Process, error) {
		root, err := m.ExtractSidecar()
		if err != nil {
			return nil, err
		}
		return m.Runner.Start(ctx, proc.Cmd{
			Name:   rt.Python,
			Args:   []string{"-u", "-m", sidecarPackage},
			Env:    m.Environ(root),
			Stderr: stderr,
		})
	}
}
