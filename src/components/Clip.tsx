"use client";
import { useId, useRef, useState } from "react";
import type { Note } from "@/lib/types";
import { api, post } from "./api";
import Spinner from "./Spinner";

/** The audio of one text: play it, or make it by generating it or uploading an MP3. `name` says what it is for, such as "the word". */
export default function Clip(props: {
  note: Note;
  slot: string;
  name: string;
  text: string;
  onNote: (n: Note) => void;
  readOnly?: boolean;
  extra?: { label: string; busy: boolean; onSelect: () => void };
  problem?: string;
}) {
  const { note, slot, name, text } = props;
  const clip = note.audio[slot];
  const has = Boolean(clip?.key);
  const needsWork = !has || clip.outdated;
  const [busy, setBusy] = useState(false);
  const [problem, setProblem] = useState("");
  const file = useRef<HTMLInputElement>(null);
  const menu = useId();
  const anchor = `--${menu.replace(/[^a-z0-9]/gi, "")}`;

  async function run(work: () => Promise<{ note: Note }>) {
    setBusy(true);
    setProblem("");
    try {
      props.onNote((await work()).note);
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

  const generateNow = () => run(() => post("/api/tts", { id: note.id, slot }));

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

  const working = busy || Boolean(props.extra?.busy);
  const canGenerate = Boolean(text.trim());
  const item = (label: string, onClick: () => void, disabled = false) => (
    <button className="options-item" popoverTarget={menu} popoverTargetAction="hide" disabled={disabled} onClick={onClick}>
      {label}
    </button>
  );

  return (
    <div role="group" aria-label={`Audio for ${name}`} className="flex flex-col items-center gap-0.5">
      <div className="flex items-center gap-2 text-sm">
        {has && (
          <button
            className={`action min-w-control-sm px-2 ${working ? "cursor-progress" : ""}`}
            aria-label={`Play ${name}`}
            aria-busy={working || undefined}
            onClick={() => !working && play()}
          >
            {working ? <Spinner size={12} /> : <svg viewBox="0 0 10 10" width="12" height="12" aria-hidden="true"><path d="M2 1l7 4-7 4z" fill="currentColor" /></svg>}
          </button>
        )}
        {has && clip.outdated && (
          <>
            <span className="text-warn">Audio is out of date</span>
            <button className="action" disabled={working || !canGenerate} onClick={generateNow}>Regenerate</button>
          </>
        )}
        <button
          className="action min-w-control-sm px-2"
          aria-label={`Audio options for ${name}`}
          aria-busy={(working && !has) || undefined}
          disabled={working}
          popoverTarget={menu}
          style={{ anchorName: anchor }}
        >
          {working && !has ? (
            <Spinner size={16} />
          ) : (
            <svg viewBox="0 0 16 4" width="16" height="4" aria-hidden="true" fill="currentColor">
              <circle cx="2" cy="2" r="1.6" /><circle cx="8" cy="2" r="1.6" /><circle cx="14" cy="2" r="1.6" />
            </svg>
          )}
        </button>
        <div id={menu} popover="auto" className="options" style={{ positionAnchor: anchor }}>
          {item(has ? "Regenerate" : "Generate", generateNow, !canGenerate)}
          {item("Upload", () => file.current?.click(), !canGenerate)}
          {has && item("Remove", () => run(() => api(`/api/audio/${note.id}/${slot}`, { method: "DELETE" })))}
          {props.extra && item(props.extra.label, props.extra.onSelect)}
        </div>
      </div>
      <span role="status" className="sr-only">{working ? "Working…" : ""}</span>
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
      <p className="min-h-[2lh] text-xs text-danger">{problem || clip?.error || props.problem}</p>
    </div>
  );
}
