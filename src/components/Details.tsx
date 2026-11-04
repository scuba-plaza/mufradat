"use client";
import { POS_LABELS, POS_VALUES, type Note, type POS } from "@/lib/types";
import InlineText from "./InlineText";

/** What a card needs rarely changed: its kind of word, root, and forms. Closed until asked for. */
export default function Details(props: {
  note: Note;
  open: boolean;
  onToggle: (open: boolean) => void;
  save: (patch: Record<string, unknown>) => void;
}) {
  const { note, save } = props;
  const setForm = (i: number, change: Partial<{ label: string; arabic: string }>) =>
    save({ forms: note.forms.map((f, j) => (j === i ? { ...f, ...change } : f)) });
  const line = "border-rule";
  return (
    <details open={props.open} onToggle={(e) => props.onToggle(e.currentTarget.open)} className="mt-12 text-sm">
      <summary className="cursor-pointer text-quiet hover:text-ink">Details</summary>
      <div className="mt-4 grid max-w-sm gap-4">
        <label className="grid gap-1">
          <span className="text-quiet">Part of speech</span>
          <select className="field" value={note.pos} onChange={(e) => save({ pos: e.target.value as POS })}>
            {POS_VALUES.map((p) => <option key={p} value={p}>{POS_LABELS[p]}</option>)}
          </select>
        </label>
        <label className="grid gap-1">
          <span className="text-quiet">Gender</span>
          <select className="field" value={note.gender} onChange={(e) => save({ gender: e.target.value })}>
            <option value="">not set</option>
            <option value="m">masculine</option>
            <option value="f">feminine</option>
            <option value="m+f">both</option>
          </select>
        </label>
        <label className="grid gap-1">
          <span className="text-quiet">Root</span>
          <InlineText label="Root" value={note.root} arabic className={`${line} text-ar-sm`} onSave={(root) => save({ root })} />
        </label>
        <label className="grid gap-1">
          <span className="text-quiet">Verb form</span>
          <InlineText label="Verb form" value={note.verbForm} className={line} onSave={(verbForm) => save({ verbForm })} />
        </label>
        <fieldset className="grid gap-2">
          <legend className="mb-1 text-quiet">Forms</legend>
          {note.forms.map((f, i) => (
            <div key={f.id} className="flex items-center gap-2">
              <div className="w-16"><InlineText label={`Label of form ${i + 1}`} value={f.label} className={line} onSave={(label) => setForm(i, { label })} /></div>
              <div className="min-w-0 flex-1"><InlineText label={`Arabic of form ${i + 1}`} value={f.arabic} arabic className={`${line} text-ar-sm`} onSave={(arabic) => setForm(i, { arabic })} /></div>
              <button className="action" aria-label={`Remove form ${i + 1}`} onClick={() => save({ forms: note.forms.filter((_, j) => j !== i) })}>Remove</button>
            </div>
          ))}
          <button className="action justify-self-start" onClick={() => save({ forms: [...note.forms, { label: "pl.", arabic: "" }] })}>Add a form</button>
        </fieldset>
      </div>
    </details>
  );
}
