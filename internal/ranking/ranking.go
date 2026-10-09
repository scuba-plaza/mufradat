package ranking

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/scuba-plaza/mufradat/data"
)

type List struct {
	ids  []string
	rank map[string]int
}

func Embedded() (*List, error) {
	return Parse(data.Ranking())
}

func Parse(r io.Reader) (*List, error) {
	l := &List{rank: map[string]int{}}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 1<<16), 1<<20)
	line := 0
	for sc.Scan() {
		line++
		text := strings.TrimSpace(sc.Text())
		if text == "" || line == 1 && strings.HasPrefix(text, "rank\t") {
			continue
		}
		cols := strings.Split(text, "\t")
		if len(cols) < 2 {
			return nil, fmt.Errorf("ranking line %d: expected rank and id", line)
		}
		n, err := strconv.Atoi(cols[0])
		if err != nil || n != len(l.ids)+1 {
			return nil, fmt.Errorf("ranking line %d: rank %q does not follow %d", line, cols[0], len(l.ids))
		}
		if _, dup := l.rank[cols[1]]; dup {
			return nil, fmt.Errorf("ranking line %d: %s is listed twice", line, cols[1])
		}
		l.ids = append(l.ids, cols[1])
		l.rank[cols[1]] = n
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return l, nil
}

func (l *List) Len() int { return len(l.ids) }

func (l *List) ID(rank int) (string, bool) {
	if rank < 1 || rank > len(l.ids) {
		return "", false
	}
	return l.ids[rank-1], true
}

func (l *List) Rank(id string) int { return l.rank[id] }

func (l *List) IDs() []string { return l.ids }
