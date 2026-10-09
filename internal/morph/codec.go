package morph

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/scuba-plaza/mufradat/internal/lexicon"
)

func (t *Table) Encode(w io.Writer) error {
	gz, err := gzip.NewWriterLevel(w, gzip.BestCompression)
	if err != nil {
		return err
	}
	bw := bufio.NewWriter(gz)
	skeletons := make([]string, 0, len(t.forms))
	for s := range t.forms {
		skeletons = append(skeletons, s)
	}
	slices.Sort(skeletons)
	for _, s := range skeletons {
		cands := slices.Clone(t.forms[s])
		slices.SortFunc(cands, func(a, b candidate) int {
			if a.lemma != b.lemma {
				return a.lemma - b.lemma
			}
			return int(a.class) - int(b.class)
		})
		bw.WriteString(s)
		bw.WriteByte('\t')
		for i, c := range cands {
			if i > 0 {
				bw.WriteByte(',')
			}
			bw.WriteString(strconv.Itoa(c.lemma))
			bw.WriteByte(':')
			bw.WriteString(strconv.Itoa(int(c.class)))
		}
		bw.WriteByte('\n')
	}
	if err := bw.Flush(); err != nil {
		return err
	}
	return gz.Close()
}

func Decode(r io.Reader, facts []Facts) (*Table, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("forms: %w", err)
	}
	defer gz.Close()
	t := NewTable(facts)
	sc := bufio.NewScanner(gz)
	sc.Buffer(make([]byte, 0, 1<<16), 1<<20)
	line := 0
	for sc.Scan() {
		line++
		skeleton, rest, ok := strings.Cut(sc.Text(), "\t")
		if !ok || skeleton == "" {
			return nil, fmt.Errorf("forms line %d: expected a skeleton and its lemmas", line)
		}
		for _, item := range strings.Split(rest, ",") {
			lemma, class, ok := strings.Cut(item, ":")
			if !ok {
				return nil, fmt.Errorf("forms line %d: %q is not lemma:class", line, item)
			}
			i, err := strconv.Atoi(lemma)
			c, cerr := strconv.Atoi(class)
			if err != nil || cerr != nil || i < 0 || i >= len(facts) || c < 0 || c > int(lexicon.Particle) {
				return nil, fmt.Errorf("forms line %d: %q is not a lemma and class of this lexicon", line, item)
			}
			t.Add(i, lexicon.Form{Skeleton: skeleton, Class: lexicon.Class(c)})
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("forms: %w", err)
	}
	return t, nil
}
