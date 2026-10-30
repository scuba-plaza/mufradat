package clips

import "context"

type SpeechRequest struct {
	Text  string
	Voice string
	Rate  float64
}

type VoiceInfo struct {
	Name   string
	Gender string
	Detail string
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
