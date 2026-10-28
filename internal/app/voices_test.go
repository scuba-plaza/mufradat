package app

import (
	"reflect"
	"testing"

	"github.com/scuba-plaza/mufradat/internal/clips"
	"github.com/scuba-plaza/mufradat/internal/review"
)

func TestVoiceOptionsForGoogleSplitTheTierFromTheLabel(t *testing.T) {
	got := voiceOptions("google", []clips.VoiceInfo{
		{Ref: "ar-XA-Chirp3-HD-Achernar", Name: "ar-XA-Chirp3-HD-Achernar", Gender: "female"},
		{Ref: "ar-XA-Wavenet-A", Name: "ar-XA-Wavenet-A", Gender: "male"},
		{Ref: "ar-XA-A", Name: "ar-XA-A", Gender: "female"},
		{Ref: "custom", Name: "custom"},
	})
	want := []review.VoiceOption{
		{Name: "ar-XA-Chirp3-HD-Achernar", Label: "Chirp3-HD-Achernar", Tier: "Chirp3", Gender: "female"},
		{Name: "ar-XA-Wavenet-A", Label: "Wavenet-A", Tier: "Wavenet", Gender: "male"},
		{Name: "ar-XA-A", Label: "ar-XA-A", Gender: "female"},
		{Name: "custom", Label: "custom"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
}

func TestVoiceOptionsForMistralShowTheDisplayNameAndHaveNoTier(t *testing.T) {
	got := voiceOptions("mistral", []clips.VoiceInfo{
		{Ref: "voice_479", Name: "Maryam", Gender: "female", Aliases: []string{"11111111-1111-4111-8111-111111111111", "voice_479"}},
		{Ref: "33333333-3333-4333-8333-333333333333", Name: "Omar", Gender: "male"},
	})
	want := []review.VoiceOption{
		{Name: "voice_479", Label: "Maryam", Gender: "female"},
		{Name: "33333333-3333-4333-8333-333333333333", Label: "Omar", Gender: "male"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
}
