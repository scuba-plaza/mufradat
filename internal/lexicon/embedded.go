package lexicon

import "github.com/scuba-plaza/mufradat/data"

func Embedded() (*Index, error) {
	return Load(data.Lexicon())
}
