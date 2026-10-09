package clips

import (
	"crypto/sha256"
	"encoding/base32"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/scuba-plaza/mufradat/internal/fsutil"
	"github.com/scuba-plaza/mufradat/internal/note"
	"github.com/scuba-plaza/mufradat/internal/tashkeel"
)

type Voice struct {
	Name string
	Rate float64
}

type Field string

const (
	Word    Field = "WordAudio"
	Forms   Field = "FormsAudio"
	Example Field = "ExampleAudio"
)

type Item struct {
	Field Field
	Text  string
	Key   string
}

var keyEncoding = base32.NewEncoding("abcdefghijklmnopqrstuvwxyz234567").WithPadding(base32.NoPadding)

const keyLength = 16

func Key(v Voice, text string) string {
	sum := sha256.Sum256([]byte("v1\x00" + v.Name + "\x00" + strconv.FormatFloat(v.Rate, 'f', -1, 64) + "\x00" + text))
	return keyEncoding.EncodeToString(sum[:])[:keyLength]
}

func Wanted(n note.Note, v Voice) []Item {
	var items []Item
	add := func(f Field, text string) {
		items = append(items, Item{Field: f, Text: text, Key: Key(v, text)})
	}
	add(Word, n.Lemma)
	if forms := n.FormsText(); forms != "" {
		add(Forms, forms)
	}
	if n.Example != "" {
		add(Example, tashkeel.Pausal(note.PlainExample(n.Example)))
	}
	return items
}

func FileName(key string) string { return key + ".mp3" }

type Store struct {
	dir string
}

func NewStore(dir string) *Store { return &Store{dir: dir} }

func (s *Store) Path(key string) string {
	return filepath.Join(s.dir, key[:2], FileName(key))
}

func (s *Store) Has(key string) bool { return fsutil.Exists(s.Path(key)) }

func (s *Store) Put(key string, data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("refusing to store an empty clip for %s", key)
	}
	return fsutil.WriteFileAtomic(s.Path(key), data, 0o644)
}

func (s *Store) Remove(key string) error {
	err := os.Remove(s.Path(key))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *Store) Keys() ([]string, error) {
	var keys []string
	err := filepath.WalkDir(s.dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".mp3") {
			return nil
		}
		keys = append(keys, strings.TrimSuffix(d.Name(), ".mp3"))
		return nil
	})
	slices.Sort(keys)
	return keys, err
}
