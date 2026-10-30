package app

import (
	"reflect"
	"testing"

	"github.com/scuba-plaza/mufradat/internal/clips"
	"github.com/scuba-plaza/mufradat/internal/review"
)

func TestVoiceOptionsSplitTheTierFromTheLabel(t *testing.T) {
	got := voiceOptions([]clips.VoiceInfo{
		{Name: "ar-XA-Chirp3-HD-Achernar", Gender: "female"},
		{Name: "ar-XA-Wavenet-A", Gender: "male"},
		{Name: "ar-XA-A", Gender: "female"},
		{Name: "custom"},
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
