package clips

import (
	"context"
	"strings"
)

type SpeechRequest struct {
	Text  string
	Voice string
	Rate  float64
}

type VoiceInfo struct {
	Ref     string
	Name    string
	Gender  string
	Detail  string
	Aliases []string
}

func (v VoiceInfo) Is(s string) bool {
	if s == "" {
		return false
	}
	if s == v.Ref {
		return true
	}
	for _, a := range v.Aliases {
		if s == a {
			return true
		}
	}
	return v.Name != "" && v.Name != v.Ref && strings.EqualFold(s, v.Name)
}

type Speaker interface {
	Synthesize(ctx context.Context, req SpeechRequest) ([]byte, error)
	Voices(ctx context.Context) ([]VoiceInfo, error)
}

type speaking struct{ speaker Speaker }

func Speaking(s Speaker) Synthesizer { return speaking{speaker: s} }

func (s speaking) Synthesize(ctx context.Context, item Item, v Voice) ([]byte, error) {
	return s.speaker.Synthesize(ctx, SpeechRequest{Text: item.Text, Voice: v.Name, Rate: v.Rate})
}
