package deck

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/scuba-plaza/mufradat/internal/apkg"
	"github.com/scuba-plaza/mufradat/internal/fsutil"
)

type State struct {
	Fingerprint string    `json:"fingerprint"`
	Path        string    `json:"path"`
	Notes       int       `json:"notes"`
	Built       time.Time `json:"built"`
}

func LoadState(path string) (State, bool, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return State{}, false, nil
	}
	if err != nil {
		return State{}, false, err
	}
	var s State
	if err := json.Unmarshal(raw, &s); err != nil {
		return State{}, false, fmt.Errorf("%s: %w", path, err)
	}
	return s, true, nil
}

func SaveState(path string, s State) error {
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, append(raw, '\n'), 0o644)
}

func Fingerprint(pkg *apkg.Package) (string, error) {
	h := sha256.New()
	put := func(parts ...string) {
		for _, p := range parts {
			io.WriteString(h, p)
			h.Write([]byte{0})
		}
	}
	put(pkg.Deck.Name, pkg.Deck.Description, pkg.Model.Name, pkg.Model.CSS, TemplateDigest())
	fmt.Fprintf(h, "%d %d %d\x00", pkg.Deck.ID, pkg.Model.ID, pkg.Created.Unix())
	for _, t := range pkg.Model.Templates {
		put(t.Name, t.Front, t.Back)
	}
	for _, n := range pkg.Notes {
		put(n.GUID)
		put(n.Fields...)
		put(n.Tags...)
		for _, c := range n.Cards {
			fmt.Fprintf(h, "%d:%d\x00", c.Ord, c.Due)
		}
		fmt.Fprintf(h, "%d\x00", n.Mod.Unix())
	}
	for _, m := range pkg.Media {
		put(m.Name)
	}
	return hex.EncodeToString(h.Sum(nil))[:16], nil
}

func Slug(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		if unicode.IsLetter(r) && r < 128 || unicode.IsDigit(r) {
			b.WriteRune(r)
			dash = false
			continue
		}
		if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
