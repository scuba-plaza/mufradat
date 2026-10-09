package pyenv

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/scuba-plaza/mufradat/internal/fsutil"
	"github.com/scuba-plaza/mufradat/internal/proc"
	"github.com/scuba-plaza/mufradat/internal/xdg"
)

const (
	readySchema    = 1
	digestLength   = 12
	lockName       = "requirements.lock"
	sidecarPackage = "mufradat_sidecar"
	MinPythonMajor = 3
	MinPythonMinor = 11
	readyFile      = "READY.json"
	probeVersionPy = "import sys; print('%d.%d.%d' % sys.version_info[:3])"
)

type CamelPackage struct {
	Name string
	Dir  string
}

var CamelPackages = []CamelPackage{
	{Name: "morphology-db-msa-r13", Dir: "morphology_db/calima-msa-r13"},
	{Name: "disambig-mle-calima-msa-r13", Dir: "disambig_mle/calima-msa-r13"},
	{Name: "disambig-bert-unfactored-msa", Dir: "disambig_bert_unfactored/msa"},
}

func camelNames() []string {
	names := make([]string, len(CamelPackages))
	for i, p := range CamelPackages {
		names[i] = p.Name
	}
	return names
}

var digestDir = regexp.MustCompile(`^[0-9a-f]{12}$`)

type Ready struct {
	Schema        int       `json:"schema"`
	LockDigest    string    `json:"lock_digest"`
	PythonVersion string    `json:"python_version"`
	CamelPackages []string  `json:"camel_packages"`
	Installer     string    `json:"installer"`
	Created       time.Time `json:"created"`
}

type Manager struct {
	Dirs   xdg.Dirs
	Source fs.FS
	Runner proc.Runner
	Now    func() time.Time
}

func (m *Manager) read(name string) ([]byte, error) { return fs.ReadFile(m.Source, name) }

func (m *Manager) LockDigest() (string, error) {
	raw, err := m.read(lockName)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])[:digestLength], nil
}

func (m *Manager) SourceDigest() (string, error) {
	files, err := fs.Glob(m.Source, path.Join(sidecarPackage, "*.py"))
	if err != nil {
		return "", err
	}
	slices.Sort(files)
	h := sha256.New()
	for _, f := range files {
		raw, err := m.read(f)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(h, "%s\x00%d\x00", f, len(raw))
		h.Write(raw)
	}
	return hex.EncodeToString(h.Sum(nil))[:digestLength], nil
}

func (m *Manager) EnvDir() (string, error) {
	d, err := m.LockDigest()
	if err != nil {
		return "", err
	}
	return filepath.Join(m.Dirs.PythonEnvs(), d), nil
}

func (m *Manager) Interpreter() (string, error) {
	dir, err := m.EnvDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "bin", "python"), nil
}

func (m *Manager) ReadyState() (Ready, bool, error) {
	dir, err := m.EnvDir()
	if err != nil {
		return Ready{}, false, err
	}
	raw, err := fsutil.ReadFileIfExists(filepath.Join(dir, readyFile))
	if err != nil || raw == nil {
		return Ready{}, false, err
	}
	var r Ready
	if err := json.Unmarshal(raw, &r); err != nil {
		return Ready{}, false, fmt.Errorf("%s: %w", filepath.Join(dir, readyFile), err)
	}
	if r.Schema != readySchema {
		return Ready{}, false, nil
	}
	return r, true, nil
}

func (m *Manager) Fingerprint(r Ready) (string, error) {
	src, err := m.SourceDigest()
	if err != nil {
		return "", err
	}
	return fingerprint(r.LockDigest, src, r.CamelPackages), nil
}

func fingerprint(lock, src string, packages []string) string {
	h := sha256.New()
	fmt.Fprintf(h, "managed\x00%s\x00%s\x00%s", lock, src, strings.Join(packages, ","))
	return hex.EncodeToString(h.Sum(nil))[:digestLength]
}

func CustomFingerprint(python, sourceDigest string) string {
	h := sha256.New()
	fmt.Fprintf(h, "custom\x00%s\x00%s", python, sourceDigest)
	return hex.EncodeToString(h.Sum(nil))[:digestLength]
}

func (m *Manager) ExtractSidecar() (string, error) {
	digest, err := m.SourceDigest()
	if err != nil {
		return "", err
	}
	root := filepath.Join(m.Dirs.Sidecar(), digest)
	pkg := filepath.Join(root, sidecarPackage)
	files, err := fs.Glob(m.Source, path.Join(sidecarPackage, "*.py"))
	if err != nil {
		return "", err
	}
	for _, f := range files {
		dest := filepath.Join(root, filepath.FromSlash(f))
		raw, err := m.read(f)
		if err != nil {
			return "", err
		}
		if cur, err := os.ReadFile(dest); err == nil && string(cur) == string(raw) {
			continue
		}
		if err := fsutil.WriteFileAtomic(dest, raw, 0o644); err != nil {
			return "", err
		}
	}
	if !fsutil.IsDir(pkg) {
		return "", fmt.Errorf("the embedded sidecar has no files")
	}
	return root, nil
}

func (m *Manager) Environ(sidecarRoot string) []string {
	cache := m.Dirs.ModelCache()
	return []string{
		"CAMELTOOLS_DATA=" + m.Dirs.CamelData(),
		"HF_HOME=" + filepath.Join(cache, "hf"),
		"TORCH_HOME=" + filepath.Join(cache, "torch"),
		"PYTHONPATH=" + sidecarRoot,
		"PYTHONNOUSERSITE=1",
		"PYTHONUNBUFFERED=1",
		"PYTHONDONTWRITEBYTECODE=1",
		"HF_HUB_OFFLINE=1",
		"TRANSFORMERS_OFFLINE=1",
		"TOKENIZERS_PARALLELISM=false",
	}
}

func (m *Manager) pruneOthers(keepEnv, keepSidecar string) []string {
	var removed []string
	clean := func(parent, keep string) {
		entries, err := os.ReadDir(parent)
		if err != nil {
			return
		}
		for _, e := range entries {
			if !e.IsDir() || e.Name() == keep || !digestDir.MatchString(e.Name()) {
				continue
			}
			if os.RemoveAll(filepath.Join(parent, e.Name())) == nil {
				removed = append(removed, filepath.Join(parent, e.Name()))
			}
		}
	}
	clean(m.Dirs.PythonEnvs(), keepEnv)
	clean(m.Dirs.Sidecar(), keepSidecar)
	return removed
}
