package verify

import (
	"slices"
	"strings"

	"github.com/scuba-plaza/mufradat/internal/tashkeel"
)

type Kind string

const (
	KindUnmarked   Kind = "unmarked"
	KindInvalid    Kind = "invalid"
	KindUnknown    Kind = "unknown"
	KindDiacritics Kind = "diacritics"
	KindUnchecked  Kind = "unchecked"
)

type Severity string

const (
	Major Severity = "major"
	Minor Severity = "minor"
)

type Issue struct {
	Field    string   `json:"field"`
	Kind     Kind     `json:"kind"`
	Severity Severity `json:"severity"`
	Word     string   `json:"word,omitempty"`
	Detail   string   `json:"detail"`
	CATT     string   `json:"catt,omitempty"`
	CAMeL    string   `json:"camel,omitempty"`
	Known    []string `json:"known,omitempty"`
	Missing  []int    `json:"missing,omitempty"`
}

func (i Issue) Key() string {
	if i.Word != "" {
		return i.Word
	}
	return i.Field + ":" + string(i.Kind)
}

const (
	knownShown   = 6
	sampleShown  = 4
	wordSeparate = "، "
)

func anyCompatible(token string, readings []string, citation bool) bool {
	for _, r := range readings {
		if citation && tashkeel.CompatibleCitation(token, r) || !citation && tashkeel.Compatible(token, r) {
			return true
		}
	}
	return false
}

func missingDetail(missing []tashkeel.MissingMark, catt, bert string) string {
	var letters []string
	ending := false
	for _, m := range missing {
		if m.Ending {
			ending = true
		} else {
			letters = append(letters, m.Letter)
		}
	}
	detail := "no vowel mark on " + strings.Join(letters, wordSeparate)
	switch {
	case len(letters) == 0:
		detail = "the ending has no vowel mark"
	case ending:
		detail += " and the ending"
	}
	if catt != "" {
		detail += "; CATT reads " + catt
	}
	if bert != "" {
		detail += "; CAMeL reads " + bert
	}
	return detail
}

func restates(token, reading string, missing []tashkeel.MissingMark) bool {
	diff := tashkeel.Differences(token, reading)
	if len(diff) == 0 || len(missing) == 0 {
		return false
	}
	for i, d := range diff {
		if d && !slices.ContainsFunc(missing, func(m tashkeel.MissingMark) bool { return m.Index == i }) {
			return false
		}
	}
	return true
}

func sample(analyses []string, n int) string {
	return strings.Join(analyses[:min(len(analyses), n)], wordSeparate)
}

func EvaluateItem(item Item, r Reading) []Issue {
	var issues []Issue
	emit := func(is Issue) { issues = append(issues, is) }
	aligned := len(r.CATT) == len(item.Tokens)
	if item.Context && !aligned {
		emit(Issue{Field: item.Field, Kind: KindUnchecked, Severity: Major, Detail: "CATT's reading could not be aligned with the sentence"})
	}
	for t, token := range item.Tokens {
		var analyses []string
		if t < len(r.Analyses) {
			analyses = r.Analyses[t]
		}
		known := len(analyses) > 0
		valid := known && anyCompatible(token, analyses, item.Citation)
		catt, bert := "", ""
		if item.Context && aligned {
			catt = r.CATT[t]
		}
		if item.Context && t < len(r.BERT) {
			bert = r.BERT[t]
		}
		whole := catt != "" && tashkeel.Compatible(token, catt)
		stem := known && anyCompatible(token, analyses, true) || catt != "" && tashkeel.CompatibleCitation(token, catt)
		missing := slices.DeleteFunc(tashkeel.UnmarkedLetters(token, item.Citation), func(m tashkeel.MissingMark) bool {
			return whole || stem && !m.Ending
		})
		if len(missing) > 0 {
			is := Issue{Field: item.Field, Kind: KindUnmarked, Severity: Major, Word: token, Detail: missingDetail(missing, catt, bert), CATT: catt, CAMeL: bert}
			for _, m := range missing {
				is.Missing = append(is.Missing, m.Index)
			}
			emit(is)
		}
		if known && !valid {
			emit(Issue{
				Field: item.Field, Kind: KindInvalid, Severity: Major, Word: token,
				Detail: "CAMeL does not allow these vowels; it knows " + sample(analyses, sampleShown),
				Known:  analyses[:min(len(analyses), knownShown)],
			})
		}
		if !item.Context {
			if !known {
				emit(Issue{Field: item.Field, Kind: KindUnknown, Severity: Minor, Word: token, Detail: "CAMeL does not know this word, so only Wiktionary vouches for it"})
			}
			continue
		}
		if catt == "" || whole || restates(token, catt, missing) {
			continue
		}
		is := Issue{Field: item.Field, Kind: KindDiacritics, Word: token, CATT: catt, CAMeL: bert}
		switch {
		case bert != "" && tashkeel.Compatible(token, bert) && valid:
			is.Severity, is.Detail = Minor, "CATT reads "+catt+"; CAMeL agrees with the card"
		case bert != "":
			is.Severity, is.Detail = Major, "CATT reads "+catt+"; CAMeL reads "+bert
		default:
			is.Severity, is.Detail = Major, "CATT reads "+catt
		}
		emit(is)
	}
	return issues
}
