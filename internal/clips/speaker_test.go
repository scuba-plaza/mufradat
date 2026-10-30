package clips

import (
	"context"
	"testing"
)

type recordingSpeaker struct {
	got []SpeechRequest
}

func (r *recordingSpeaker) Synthesize(_ context.Context, req SpeechRequest) ([]byte, error) {
	r.got = append(r.got, req)
	return []byte("audio"), nil
}

func (r *recordingSpeaker) Voices(context.Context) ([]VoiceInfo, error) { return nil, nil }

func TestSpeakingPassesTheTextVoiceAndRateOfTheClip(t *testing.T) {
	sp := &recordingSpeaker{}
	synth := Speaking(sp)
	item := Item{Field: Word, Text: "كِتَاب", Key: "k"}
	audio, err := synth.Synthesize(context.Background(), item, Voice{Name: "ar-XA-A", Rate: 0.8})
	if err != nil || string(audio) != "audio" {
		t.Fatalf("%q, %v", audio, err)
	}
	if len(sp.got) != 1 || sp.got[0] != (SpeechRequest{Text: "كِتَاب", Voice: "ar-XA-A", Rate: 0.8}) {
		t.Fatalf("request %+v", sp.got)
	}
}
