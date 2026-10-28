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
	audio, err := synth.Synthesize(context.Background(), item, Voice{Provider: "google", Name: "ar-XA-A", Rate: 0.8})
	if err != nil || string(audio) != "audio" {
		t.Fatalf("%q, %v", audio, err)
	}
	if len(sp.got) != 1 || sp.got[0] != (SpeechRequest{Text: "كِتَاب", Voice: "ar-XA-A", Rate: 0.8}) {
		t.Fatalf("request %+v", sp.got)
	}
}

func TestVoiceInfoMatchesAnySpellingTheUserMayHaveWritten(t *testing.T) {
	v := VoiceInfo{Ref: "voice_479", Name: "Maryam", Aliases: []string{"11111111-1111-4111-8111-111111111111"}}
	for _, s := range []string{"voice_479", "Maryam", "maryam", "11111111-1111-4111-8111-111111111111"} {
		if !v.Is(s) {
			t.Errorf("%q should match", s)
		}
	}
	for _, s := range []string{"", "voice_480", "Mary"} {
		if v.Is(s) {
			t.Errorf("%q should not match", s)
		}
	}
	if !(VoiceInfo{Ref: "ar-XA-Wavenet-A"}).Is("ar-XA-Wavenet-A") {
		t.Error("a voice without aliases still matches its own reference")
	}
	if (VoiceInfo{Ref: "ar-XA-Wavenet-A"}).Is("ar-xa-wavenet-a") {
		t.Error("references are case sensitive; only the display name is matched loosely")
	}
}
