package status

import "fmt"

type StepStatus string

const (
	Done    StepStatus = "done"
	Todo    StepStatus = "todo"
	Skipped StepStatus = "skipped"
)

type Step struct {
	Name    string     `json:"name"`
	Status  StepStatus `json:"status"`
	Detail  string     `json:"detail"`
	Command string     `json:"command,omitempty"`
}

type Snapshot struct {
	Prefix        string
	Notes         int
	InvalidNotes  int
	RankedTotal   int
	RankedWritten int

	VerifyEnabled  bool
	VerifyRequired bool
	VerifyReason   string
	Unchecked      int
	OpenMajor      int
	OpenMinor      int

	AudioReady   bool
	AudioReason  string
	AudioVoice   string
	ClipsWanted  int
	ClipsMissing int

	Built        bool
	BuildCurrent bool
	Package      string
}

type State struct {
	Steps []Step `json:"steps"`
	Next  string `json:"next"`
	Done  bool   `json:"done"`
}

func Compute(s Snapshot) State {
	cmd := func(sub string) string { return s.Prefix + " " + sub }
	var steps []Step

	add := Step{Name: "add", Command: cmd("add")}
	switch s.Notes {
	case 0:
		add.Status, add.Detail = Todo, "no notes yet"
	default:
		add.Status = Done
		add.Detail = fmt.Sprintf("%d notes (%d of the %d most common words)", s.Notes, s.RankedWritten, s.RankedTotal)
		if s.InvalidNotes > 0 {
			add.Status = Todo
			add.Detail += fmt.Sprintf("; %d fail validation", s.InvalidNotes)
			add.Command = cmd("review --all")
		}
	}
	steps = append(steps, add)

	check := Step{Name: "check", Command: cmd("check")}
	switch {
	case s.Notes == 0:
		check.Status, check.Detail = Skipped, "nothing to check"
	case !s.VerifyEnabled && s.VerifyRequired:
		check.Status, check.Detail = Todo, s.VerifyReason
		check.Command = cmd("setup")
	case !s.VerifyEnabled:
		check.Status, check.Detail = Skipped, s.VerifyReason
		check.Command = cmd("setup")
	case s.Unchecked > 0:
		check.Status, check.Detail = Todo, fmt.Sprintf("%d notes not checked yet", s.Unchecked)
	default:
		check.Status, check.Detail = Done, "every note is checked"
	}
	steps = append(steps, check)

	review := Step{Name: "review", Command: cmd("review")}
	switch {
	case s.Notes == 0 || !s.VerifyEnabled:
		review.Status, review.Detail = Skipped, "nothing to review"
	case s.Unchecked > 0:
		review.Status, review.Detail = Skipped, "check first"
	case s.OpenMajor+s.OpenMinor > 0:
		review.Status = Todo
		review.Detail = fmt.Sprintf("%d notes with open flags (%d major, %d minor)", s.OpenMajor+s.OpenMinor, s.OpenMajor, s.OpenMinor)
	default:
		review.Status, review.Detail = Done, "no open flags"
	}
	steps = append(steps, review)

	audio := Step{Name: "audio", Command: cmd("audio")}
	switch {
	case s.Notes == 0:
		audio.Status, audio.Detail = Skipped, "nothing to speak"
	case !s.AudioReady:
		audio.Status, audio.Detail = Skipped, s.AudioReason
		audio.Command = cmd("auth google --import KEY.json")
	case s.ClipsMissing > 0:
		audio.Status = Todo
		audio.Detail = fmt.Sprintf("%d of %d clips missing", s.ClipsMissing, s.ClipsWanted) + voiceNote(s.AudioVoice)
	default:
		audio.Status, audio.Detail = Done, fmt.Sprintf("%d clips", s.ClipsWanted)+voiceNote(s.AudioVoice)
	}
	steps = append(steps, audio)

	build := Step{Name: "build", Command: cmd("build")}
	switch {
	case s.Notes == 0:
		build.Status, build.Detail = Skipped, "nothing to build"
	case !s.Built:
		build.Status, build.Detail = Todo, "no package yet"
	case !s.BuildCurrent:
		build.Status, build.Detail = Todo, "the package is out of date"
	default:
		build.Status, build.Detail = Done, s.Package
	}
	steps = append(steps, build)

	state := State{Steps: steps}
	for _, st := range steps {
		if st.Status == Todo {
			state.Next = st.Command
			return state
		}
	}
	state.Done = true
	state.Next = cmd("add")
	return state
}

func voiceNote(voice string) string {
	if voice == "" {
		return ""
	}
	return " with " + voice
}
