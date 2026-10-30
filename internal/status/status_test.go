package status

import "testing"

func snap() Snapshot {
	return Snapshot{
		Prefix: "mufradat", Notes: 10, RankedTotal: 5000, RankedWritten: 10,
		VerifyEnabled: true, AudioReady: true, ClipsWanted: 30, Built: true, BuildCurrent: true, Package: "out/x.apkg",
	}
}

func stepOf(s State, name string) Step {
	for _, st := range s.Steps {
		if st.Name == name {
			return st
		}
	}
	return Step{}
}

func TestEmptyWorkspaceNamesAdd(t *testing.T) {
	st := Compute(Snapshot{Prefix: "mufradat", VerifyEnabled: true, AudioReady: true})
	if st.Next != "mufradat add" || st.Done {
		t.Errorf("next = %q done=%v", st.Next, st.Done)
	}
	for _, name := range []string{"check", "review", "audio", "build"} {
		if stepOf(st, name).Status != Skipped {
			t.Errorf("%s should be skipped with no notes", name)
		}
	}
}

func TestPipelineOrder(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Snapshot)
		next   string
	}{
		{"unchecked", func(s *Snapshot) { s.Unchecked = 3 }, "mufradat check"},
		{"open flags", func(s *Snapshot) { s.OpenMajor = 1 }, "mufradat review"},
		{"missing audio", func(s *Snapshot) { s.ClipsMissing = 4 }, "mufradat audio"},
		{"stale build", func(s *Snapshot) { s.BuildCurrent = false }, "mufradat build"},
		{"never built", func(s *Snapshot) { s.Built = false }, "mufradat build"},
		{"invalid notes", func(s *Snapshot) { s.InvalidNotes = 2 }, "mufradat review --all"},
	}
	for _, c := range cases {
		s := snap()
		c.mutate(&s)
		if got := Compute(s); got.Next != c.next || got.Done {
			t.Errorf("%s: next = %q done=%v, want %q", c.name, got.Next, got.Done, c.next)
		}
	}
}

func TestEverythingDone(t *testing.T) {
	st := Compute(snap())
	if !st.Done || st.Next != "mufradat add" {
		t.Errorf("state = %+v", st)
	}
}

func TestOptionalStagesAreSkippedNotBlocking(t *testing.T) {
	s := snap()
	s.VerifyEnabled, s.VerifyReason = false, "verification is not set up"
	s.AudioReady, s.AudioReason = false, "no Google credentials"
	st := Compute(s)
	if stepOf(st, "check").Status != Skipped || stepOf(st, "audio").Status != Skipped {
		t.Errorf("steps = %+v", st.Steps)
	}
	if !st.Done {
		t.Errorf("skipped optional stages must not block: %+v", st)
	}
	if stepOf(st, "check").Command != "mufradat setup" {
		t.Errorf("check should point at setup: %q", stepOf(st, "check").Command)
	}
}

func TestUncheckedHidesReview(t *testing.T) {
	s := snap()
	s.Unchecked, s.OpenMajor = 2, 5
	if got := stepOf(Compute(s), "review"); got.Status != Skipped {
		t.Errorf("review = %+v", got)
	}
}

func TestTheAudioStepNamesTheVoiceItSpeaksWith(t *testing.T) {
	s := snap()
	s.AudioVoice = "ar-XA-Chirp3-HD-Achernar (rate 0.9)"
	if got := stepOf(Compute(s), "audio").Detail; got != "30 clips with ar-XA-Chirp3-HD-Achernar (rate 0.9)" {
		t.Errorf("done: %q", got)
	}
	s.ClipsMissing = 4
	if got := stepOf(Compute(s), "audio").Detail; got != "4 of 30 clips missing with ar-XA-Chirp3-HD-Achernar (rate 0.9)" {
		t.Errorf("missing: %q", got)
	}
	s.AudioVoice = ""
	if got := stepOf(Compute(s), "audio").Detail; got != "4 of 30 clips missing" {
		t.Errorf("without a voice: %q", got)
	}
}

func TestAudioThatCannotBeMadeSendsYouToTheGoogleSetup(t *testing.T) {
	s := snap()
	s.ClipsMissing, s.AudioReady, s.AudioReason = 4, false, "no Google credentials"
	step := stepOf(Compute(s), "audio")
	if step.Status != Skipped || step.Detail != "no Google credentials" || step.Command != "mufradat auth google --import KEY.json" {
		t.Errorf("got %+v", step)
	}
}
