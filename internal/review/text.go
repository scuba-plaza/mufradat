package review

import (
	"strings"

	"github.com/scuba-plaza/mufradat/internal/arabic"
	"github.com/scuba-plaza/mufradat/internal/tashkeel"
)

type piece struct {
	runes  []rune
	target []bool
	word   bool
}

func (p piece) String() string {
	return string(p.runes)
}

func wordRune(r rune) bool {
	return arabic.IsLetter(r) || tashkeel.IsMark(r) || r == tashkeel.Tatweel
}

func pieces(markup string) []piece {
	var out []piece
	target := false
	for _, r := range markup {
		switch r {
		case '[':
			target = true
			continue
		case ']':
			target = false
			continue
		}
		w := wordRune(r)
		if n := len(out); n > 0 && out[n-1].word == w {
			out[n-1].runes = append(out[n-1].runes, r)
			out[n-1].target = append(out[n-1].target, target)
			continue
		}
		out = append(out, piece{runes: []rune{r}, target: []bool{target}, word: w})
	}
	return out
}

func markupOf(ps []piece) string {
	var b strings.Builder
	open := false
	for _, p := range ps {
		for k, r := range p.runes {
			if p.target[k] != open {
				open = p.target[k]
				if open {
					b.WriteByte('[')
				} else {
					b.WriteByte(']')
				}
			}
			b.WriteRune(r)
		}
	}
	if open {
		b.WriteByte(']')
	}
	return b.String()
}

func words(ps []piece) []string {
	var out []string
	for _, p := range ps {
		if p.word {
			out = append(out, p.String())
		}
	}
	return out
}

func common(a, b []string) ([]bool, []bool) {
	n, m := len(a), len(b)
	length := make([][]int, n+1)
	for i := range length {
		length[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				length[i][j] = length[i+1][j+1] + 1
			} else {
				length[i][j] = max(length[i+1][j], length[i][j+1])
			}
		}
	}
	inA, inB := make([]bool, n), make([]bool, m)
	for i, j := 0, 0; i < n && j < m; {
		switch {
		case a[i] == b[j]:
			inA[i], inB[j] = true, true
			i++
			j++
		case length[i+1][j] >= length[i][j+1]:
			i++
		default:
			j++
		}
	}
	return inA, inB
}
