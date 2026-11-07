"use client";
import { useEffect, useRef, useState, type ReactNode } from "react";
import type { Note } from "@/lib/types";
import { api, post } from "./api";
import Spinner from "./Spinner";

const ICONS = {
  play: <path d="M7 4l13 8-13 8z" fill="currentColor" stroke="none" />,
  generate: <><path d="M11 5L6 9H2v6h4l5 4V5z" /><path d="M15.5 8.5a5 5 0 0 1 0 7" /><path d="M19 5a10 10 0 0 1 0 14" /></>,
  regenerate: <><path d="M21 4v6h-6" /><path d="M3 20v-6h6" /><path d="M3.5 9a9 9 0 0 1 14.9-3.4L21 10" /><path d="M3 14l2.6 4.4A9 9 0 0 0 20.5 15" /></>,
  upload: <><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" /><path d="M17 8l-5-5-5 5" /><path d="M12 3v12" /></>,
  remove: <><path d="M3 6h18" /><path d="M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6" /><path d="M10 11v6" /><path d="M14 11v6" /><path d="M9 6V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2" /></>,
  newExample: <><path d="M12 3l1.9 5.1L19 10l-5.1 1.9L12 17l-1.9-5.1L5 10l5.1-1.9z" /><path d="M19 3v4" /><path d="M21 5h-4" /></>,
};

function RailButton(props: { icon: keyof typeof ICONS; title: string; onClick: () => void; disabled?: boolean; busy?: boolean }) {
  return (
    <button
      type="button"
      title={props.title}
      disabled={props.disabled}
      aria-busy={props.busy || undefined}
      onClick={props.onClick}
      className={`inline-flex h-[24px] w-[24px] lg:h-[32px] lg:w-[32px] items-center justify-center rounded-full text-ink transition-colors hover:bg-hover disabled:cursor-not-allowed disabled:opacity-40 ${props.busy ? "cursor-progress" : ""}`}
    >
      {props.busy ? (
        <Spinner size={16} />
      ) : (
        <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true" focusable="false" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          {ICONS[props.icon]}
        </svg>
      )}
    </button>
  );
}

export default function Clip(props: {
  note: Note;
  slot: string;
  name: string;
  text: string;
  onNote: (n: Note) => void;
  readOnly?: boolean;
  extra?: { label: string; busy: boolean; onSelect: () => void };
  problem?: string;
  onMessage?: (message: string) => void;
  children?: ReactNode;
}) {
  const { note, slot, name, text } = props;
  const clip = note.audio[slot];
  const has = Boolean(clip?.key);
  const [busy, setBusy] = useState(false);
  const [problem, setProblem] = useState("");
  const file = useRef<HTMLInputElement>(null);
  const state = !has ? "no audio" : clip.outdated ? "audio is out of date" : "";

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

  const message = problem || clip?.error || props.problem || "";
  const { onMessage } = props;
  useEffect(() => onMessage?.(message), [onMessage, message]);

  if (props.readOnly) {
    return (
      <span role="group" aria-label={`Audio for ${name}`} className="inline-flex min-h-control-sm items-center gap-1">
        {has && (
          <button className="action-quiet min-w-control-sm px-2" aria-label={`Play ${name}`} onClick={play}>
            <svg viewBox="0 0 10 10" width="12" height="12" aria-hidden="true"><path d="M2 1l7 4-7 4z" fill="currentColor" /></svg>
          </button>
        )}
        {state && <span className="sr-only">{state}</span>}
        {clip?.error && <span className="text-xs text-danger">{clip.error}</span>}
        {problem && <span role="alert" className="text-xs text-danger">{problem}</span>}
      </span>
    );
  }

  const working = busy || Boolean(props.extra?.busy);
  const canGenerate = Boolean(text.trim());
  const tone = message ? "border-danger" : clip?.outdated ? "border-warn" : has ? "border-rule-strong" : "border-dashed border-rule-strong";

  return (
    <div>
      <div className="grid grid-cols-1 items-center sm:grid-cols-[2.5rem_minmax(0,1fr)_2.5rem] lg:grid-cols-[3.2rem_minmax(0,1fr)_3.2rem]">
        <div className={`row-start-2 flex flex-col justify-center sm:col-start-2 sm:row-start-1 ${props.extra ? "sm:min-h-[137px] lg:min-h-[181px]" : "sm:min-h-[110px] lg:min-h-[146px]"}`}>{props.children}</div>
        <div
          role="group"
          aria-label={`Audio for ${name}`}
          className={`row-start-1 m-1 flex items-center justify-self-center sm:col-start-3 sm:flex-col sm:justify-self-end rounded-full border bg-paper p-0.5 ${tone}`}
        >
          <RailButton icon="play" title={`Play ${name}`} disabled={!has && !working} busy={working} onClick={() => !working && play()} />
          <RailButton
            icon={has ? "regenerate" : "generate"}
            title={clip?.outdated ? "Regenerate (the Arabic changed)" : has ? "Regenerate" : "Generate"}
            disabled={working || !canGenerate}
            onClick={generateNow}
          />
          <RailButton icon="upload" title="Upload" disabled={working || !canGenerate} onClick={() => file.current?.click()} />
          <RailButton icon="remove" title="Remove" disabled={working || !has} onClick={() => run(() => api(`/api/audio/${note.id}/${slot}`, { method: "DELETE" }))} />
          {props.extra && (
            <>
              <span aria-hidden="true" className="mx-px h-4 w-px bg-rule-strong/60 sm:mx-0 sm:my-px sm:h-px sm:w-4" />
              <RailButton icon="newExample" title={props.extra.label} disabled={working} onClick={props.extra.onSelect} />
            </>
          )}
          {state && <span className="sr-only">{state}</span>}
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
        </div>
      </div>
      {!onMessage && <p className="min-h-[2lh] text-xs text-danger">{message}</p>}
    </div>
  );
}
