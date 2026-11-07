"use client";
import { POS_LABELS, POS_VALUES, type Note, type POS } from "@/lib/types";
import InlineText from "./InlineText";

export default function Details(props: {
  note: Note;
  save: (patch: Record<string, unknown>) => void;
}) {
  const { note, save } = props;
  return (
    <div className="grid max-w-sm gap-4">
      <label className="grid gap-1">
        <span className="text-sm text-quiet">Part of speech</span>
        <select className="field" value={note.pos} onChange={(e) => save({ pos: e.target.value as POS })}>
          {POS_VALUES.map((p) => <option key={p} value={p}>{POS_LABELS[p]}</option>)}
        </select>
      </label>
      <label className="grid gap-1">
        <span className="text-sm text-quiet">Gender</span>
        <select className="field" value={note.gender} onChange={(e) => save({ gender: e.target.value })}>
          <option value="">not set</option>
          <option value="m">masculine</option>
          <option value="f">feminine</option>
          <option value="m+f">both</option>
        </select>
      </label>
      <label className="grid gap-1">
        <span className="text-sm text-quiet">Root</span>
        <InlineText label="Root" value={note.root} arabic className="text-ar-sm" onSave={(root) => save({ root })} />
      </label>
      <label className="grid gap-1">
        <span className="text-sm text-quiet">Verb form</span>
        <InlineText label="Verb form" value={note.verbForm} onSave={(verbForm) => save({ verbForm })} />
      </label>
    </div>
  );
}
