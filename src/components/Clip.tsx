"use client";
import { useRef, useState } from "react";
import type { Note } from "@/lib/types";
import { api, post } from "./api";

/** The audio of one text: play it, or make it by generating it or uploading an MP3. `name` says what it is for, such as "the word". */
export default function Clip(props: {
  note: Note;
  slot: string;
  name: string;
  text: string;
  onNote: (n: Note) => void;
  readOnly?: boolean;
}) {
  const { note, slot, name, text } = props;
  const clip = note.audio[slot];
  const has = Boolean(clip?.key);
  const needsWork = !has || clip.outdated;
  const [busy, setBusy] = useState(false);
  const [problem, setProblem] = useState("");
  const [changing, setChanging] = useState(false);
  const file = useRef<HTMLInputElement>(null);

  async function run(work: () => Promise<{ note: Note }>) {
    setBusy(true);
    setProblem("");
    try {
      props.onNote((await work()).note);
      setChanging(false);
    } catch (e) {
      setProblem((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  function play() {
    setProblem("");
    const cannot = () => setProblem("This clip could not be played.");
    const audio = new Audio(`/api/audio/${note.id}/${slot}?k=${clip.key}`);
    audio.onerror = cannot;
    audio.play().catch(cannot);
  }

  const generate = (
    <button className="action" disabled={busy || !text.trim()} onClick={() => run(() => post("/api/tts", { id: note.id, slot }))}>
      {busy ? "Working…" : has ? "Generate again" : "Generate"}
    </button>
  );
  const upload = (
    <button className="action" disabled={busy} onClick={() => file.current?.click()}>Upload</button>
  );

  if (props.readOnly) {
    return (
      <span role="group" aria-label={`Audio for ${name}`} className="inline-flex min-h-control-sm items-center gap-1">
        {has && (
          <button className="action-quiet min-w-control-sm px-2" aria-label={`Play ${name}`} onClick={play}>
            <svg viewBox="0 0 10 10" width="12" height="12" aria-hidden="true"><path d="M2 1l7 4-7 4z" fill="currentColor" /></svg>
          </button>
        )}
        {needsWork && <span className="text-xs text-warn">{has ? "outdated" : "no audio"}</span>}
        {problem && <span role="alert" className="text-xs text-danger">{problem}</span>}
      </span>
    );
  }

  return (
    <div role="group" aria-label={`Audio for ${name}`} className="flex flex-col items-center gap-0.5">
      <div className="flex items-center gap-2 text-sm">
        {has && (
          <button className="action min-w-control-sm px-2" aria-label={`Play ${name}`} onClick={play}>
            <svg viewBox="0 0 10 10" width="12" height="12" aria-hidden="true"><path d="M2 1l7 4-7 4z" fill="currentColor" /></svg>
          </button>
        )}
        {needsWork ? (
          <>
            {has && <span className="text-warn">Audio is out of date</span>}
            {generate}{upload}
          </>
        ) : (
          <>
            <button className="action" aria-expanded={changing} onClick={() => setChanging(!changing)}>Change</button>
            {changing && <>{generate}{upload}</>}
          </>
        )}
      </div>
      <input
        ref={file}
        type="file"
        accept="audio/mpeg,.mp3"
        className="hidden"
        onChange={(e) => {
          const f = e.target.files?.[0];
          e.target.value = "";
          e.target.blur();
          if (f) run(() => api(`/api/audio/${note.id}/${slot}`, { method: "PUT", body: f }));
        }}
      />
      {(problem || clip?.error) && <p className="text-sm text-danger">{problem || clip?.error}</p>}
    </div>
  );
}
