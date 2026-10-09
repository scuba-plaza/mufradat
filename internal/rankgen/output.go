package rankgen

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/scuba-plaza/mufradat/internal/jsonl"
	"github.com/scuba-plaza/mufradat/internal/lexicon"
)

const glossRunes = 80

func trim(l *lexicon.Lemma, maxSenses int) *lexicon.Lemma {
	out := &lexicon.Lemma{ID: l.ID}
	for _, e := range l.Entries {
		if e.Pos == "name" {
			continue
		}
		senses := e.MSASenses()
		if len(senses) == 0 {
			continue
		}
		if maxSenses > 0 && len(senses) > maxSenses {
			senses = senses[:maxSenses]
		}
		c := *e
		c.Senses = senses
		out.Entries = append(out.Entries, &c)
	}
	if len(out.Entries) == 0 {
		return nil
	}
	return out
}

func Ship(rows []Row, all []*lexicon.Lemma, maxSenses int) ([]*lexicon.Lemma, error) {
	taken := map[*lexicon.Lemma]bool{}
	var out []*lexicon.Lemma
	for _, r := range rows {
		l := trim(r.Lemma, maxSenses)
		if l == nil {
			return nil, fmt.Errorf("ranked lemma %s has no MSA sense to ship", r.Lemma.ID)
		}
		taken[r.Lemma] = true
		out = append(out, l)
	}
	var rest []*lexicon.Lemma
	for _, l := range all {
		if taken[l] || !l.MSA() {
			continue
		}
		if t := trim(l, maxSenses); t != nil {
			rest = append(rest, t)
		}
	}
	slices.SortFunc(rest, func(a, b *lexicon.Lemma) int { return strings.Compare(a.ID, b.ID) })
	return append(out, rest...), nil
}

func WriteRanking(w io.Writer, ranked []*lexicon.Lemma) error {
	bw := bufio.NewWriter(w)
	bw.WriteString("rank\tid\tpos\tgloss\n")
	flat := strings.NewReplacer("\t", " ", "\n", " ")
	for i, l := range ranked {
		gloss := []rune(flat.Replace(l.Gloss()))
		text := string(gloss)
		if len(gloss) > glossRunes {
			text = string(gloss[:glossRunes]) + "…"
		}
		fmt.Fprintf(bw, "%d\t%s\t%s\t%s\n", i+1, l.ID, strings.Join(l.Pos(), ","), text)
	}
	return bw.Flush()
}

func WriteLexicon(w io.Writer, lemmas []*lexicon.Lemma) error {
	gz, err := gzip.NewWriterLevel(w, gzip.BestCompression)
	if err != nil {
		return err
	}
	if err := jsonl.Encode(gz, lemmas); err != nil {
		return err
	}
	return gz.Close()
}

func WriteForms(w io.Writer, lemmas []*lexicon.Lemma) error {
	return NewIndex(lemmas).table.Encode(w)
}

func WriteDetail(w io.Writer, rows []Row, corpora []string) error {
	bw := bufio.NewWriter(w)
	header := []string{"rank", "id", "pos", "score"}
	for _, c := range corpora {
		header = append(header, c+"_ppm")
	}
	header = append(header, "agree", "contested", "essential", "gloss")
	bw.WriteString(strings.Join(header, "\t") + "\n")
	for _, r := range rows {
		cols := []string{strconv.Itoa(r.Rank), r.Lemma.ID, strings.Join(r.Lemma.Pos(), ","), fmt.Sprintf("%.2f", r.Score)}
		for _, c := range corpora {
			cols = append(cols, fmt.Sprintf("%.2f", r.PPM[c]))
		}
		essential := ""
		if r.Essential > 0 {
			essential = strconv.Itoa(r.Essential)
		}
		cols = append(cols, fmt.Sprintf("%.2f", r.Agree), fmt.Sprintf("%.2f", r.Contested), essential,
			strings.NewReplacer("\t", " ", "\n", " ").Replace(r.Lemma.Gloss()))
		bw.WriteString(strings.Join(cols, "\t") + "\n")
	}
	return bw.Flush()
}

func WriteDecisions(w io.Writer, ix *Index, decisions []Decision, limit int, outcomes map[string]bool) error {
	bw := bufio.NewWriter(w)
	bw.WriteString("surface\tppm\toutcome\tcamel\twiktionary\tcandidates\n")
	name := func(i int) string {
		if i < 0 {
			return ""
		}
		return ix.Lemmas[i].ID
	}
	n := 0
	for _, d := range decisions {
		if len(outcomes) > 0 && !outcomes[d.Outcome] {
			continue
		}
		if limit > 0 && n >= limit {
			break
		}
		n++
		var cands []string
		for _, c := range ix.Analyze(d.Surface) {
			cands = append(cands, ix.Lemmas[c].ID+"/"+ix.Lemmas[c].Main().Pos)
		}
		fmt.Fprintf(bw, "%s\t%.1f\t%s\t%s\t%s\t%s\n", d.Surface, d.PPM, d.Outcome, name(d.A), name(d.B), strings.Join(cands, " "))
	}
	return bw.Flush()
}
