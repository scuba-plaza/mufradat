package author

import (
	"fmt"
	"strings"

	"github.com/scuba-plaza/mufradat/internal/note"
	"github.com/scuba-plaza/mufradat/internal/vocab"
)

type Vocabulary interface {
	Beyond(example string, ceiling int) []vocab.Offender
}

func (a Author) checkVocabulary(n note.Note) error {
	if a.Vocabulary == nil || a.AllowRank <= 0 {
		return nil
	}
	off := a.Vocabulary.Beyond(n.Example, max(a.AllowRank, n.Rank))
	if len(off) == 0 {
		return nil
	}
	words := make([]string, len(off))
	for i, o := range off {
		words[i] = o.String()
	}
	return fmt.Errorf("the example uses words the learner does not know yet: %s; write the sentence again with only common words", strings.Join(words, "; "))
}
