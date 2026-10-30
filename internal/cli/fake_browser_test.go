package cli_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

type browserEntry struct {
	ID       string         `json:"id"`
	Rev      string         `json:"rev"`
	State    string         `json:"state"`
	Checking bool           `json:"checking"`
	Note     map[string]any `json:"note"`
}

type browserView struct {
	List []struct {
		ID    string `json:"id"`
		State string `json:"state"`
	} `json:"list"`
	Entry *browserEntry `json:"entry"`
}

func browserState(base, id string) (browserView, error) {
	var v browserView
	resp, err := http.Get(base + "api/state?id=" + url.QueryEscape(id))
	if err != nil {
		return v, err
	}
	defer resp.Body.Close()
	return v, json.NewDecoder(resp.Body).Decode(&v)
}

func browserPost(base, action, id string, body map[string]any) (int, string, error) {
	raw, _ := json.Marshal(body)
	resp, err := http.Post(base+"api/"+action+"?id="+url.QueryEscape(id), "application/json", bytes.NewReader(raw))
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(out), nil
}

func fakeBrowser() int {
	log := os.Getenv("FAKE_BROWSER_LOG")
	fail := func(format string, args ...any) int {
		logLine(log, "error: "+fmt.Sprintf(format, args...))
		return 1
	}
	if len(os.Args) < 2 {
		return fail("no address")
	}
	base := os.Args[1]
	mode := os.Getenv("FAKE_BROWSER")
	logLine(log, "opened "+mode)
	if mode == "compare" || mode == "compare-keep" {
		return fakeComparer(base, log, mode == "compare")
	}

	list, err := browserState(base, "")
	if err != nil {
		return fail("%v", err)
	}
	id := ""
	for _, item := range list.List {
		if item.State == "open" {
			id = item.ID
			break
		}
	}
	if id != "" && mode != "finish" {
		v, err := browserState(base, id)
		if err != nil || v.Entry == nil {
			return fail("no entry for %s: %v", id, err)
		}
		e := v.Entry
		switch mode {
		case "keep", "keep-undo":
			code, body, err := browserPost(base, "keep", id, map[string]any{"if_rev": e.Rev})
			logLine(log, fmt.Sprintf("keep %d %v %s", code, err, body))
			if mode == "keep-undo" {
				code, body, err := browserPost(base, "undo", "", nil)
				logLine(log, fmt.Sprintf("undo %d %v %s", code, err, body))
			}
		case "edit":
			note := map[string]any{}
			for _, k := range []string{"lemma", "pos", "gender", "verb_form", "root", "forms", "english", "hint", "example", "example_en", "comment"} {
				if v, ok := e.Note[k]; ok {
					note[k] = v
				}
			}
			note["hint"] = "edited in review"
			code, body, err := browserPost(base, "save", id, map[string]any{"if_rev": e.Rev, "note": note})
			logLine(log, fmt.Sprintf("save %d %v %s", code, err, body))
			for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
				after, err := browserState(base, id)
				if err == nil && after.Entry != nil && !after.Entry.Checking {
					logLine(log, "settled "+after.Entry.State)
					break
				}
			}
		default:
			return fail("unknown mode %q", mode)
		}
	}
	code, body, err := browserPost(base, "finish", "", nil)
	logLine(log, fmt.Sprintf("finish %d %v %s", code, err, body))
	return 0
}

func fakeComparer(base, log string, use bool) int {
	resp, err := http.Get(base + "api/compare")
	if err != nil {
		logLine(log, "error: "+err.Error())
		return 1
	}
	var view struct {
		Voice   string `json:"voice"`
		Samples []struct {
			ID    string `json:"id"`
			Label string `json:"label"`
		} `json:"samples"`
		Voices []struct {
			Name string `json:"name"`
			Tier string `json:"tier"`
		} `json:"voices"`
	}
	err = json.NewDecoder(resp.Body).Decode(&view)
	resp.Body.Close()
	if err != nil || len(view.Voices) == 0 || len(view.Samples) == 0 {
		logLine(log, fmt.Sprintf("error: compare view %v %+v", err, view))
		return 1
	}
	logLine(log, fmt.Sprintf("compare current=%s voices=%d samples=%d", view.Voice, len(view.Voices), len(view.Samples)))
	target := ""
	for _, v := range view.Voices {
		if v.Name != view.Voice {
			target = v.Name
		}
	}
	for _, s := range view.Samples {
		resp, err := http.Get(base + "sample?voice=" + url.QueryEscape(target) + "&id=" + url.QueryEscape(s.ID))
		if err != nil {
			logLine(log, "error: "+err.Error())
			return 1
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		logLine(log, fmt.Sprintf("sample %s %d %s", s.Label, resp.StatusCode, body))
	}
	if use {
		code, body, err := browserPost(base, "use", "", map[string]any{"voice": target})
		logLine(log, fmt.Sprintf("use %d %v %s", code, err, body))
	}
	code, body, err := browserPost(base, "finish", "", nil)
	logLine(log, fmt.Sprintf("finish %d %v %s", code, err, body))
	return 0
}
