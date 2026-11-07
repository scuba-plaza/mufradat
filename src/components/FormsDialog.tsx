"use client";
import { useEffect, useRef } from "react";
import type { Note } from "@/lib/types";
import Clip from "./Clip";
import InlineText from "./InlineText";

export default function FormsDialog(props: {
  note: Note;
  open: boolean;
  onClose: () => void;
  save: (patch: Record<string, unknown>) => void;
  onNote: (n: Note) => void;
}) {
  const { note, save } = props;
  const dialog = useRef<HTMLDialogElement>(null);

  useEffect(() => {
    const el = dialog.current;
    if (!el) return;
    if (props.open && !el.open) el.showModal();
    if (!props.open && el.open) el.close();
  }, [props.open]);

  const setForm = (i: number, change: Partial<{ label: string; arabic: string }>) =>
    save({ forms: note.forms.map((f, j) => (j === i ? { ...f, ...change } : f)) });

  return (
    <dialog
      ref={dialog}
      aria-labelledby="forms-title"
      onClose={props.onClose}
      onClick={(e) => e.target === dialog.current && props.onClose()}
      className="max-h-[90vh] w-[calc(100%-2.5rem)] max-w-column overflow-y-auto rounded-modal border border-rule bg-surface p-0 text-ink shadow-modal backdrop:bg-black/70"
    >
      <div className="p-6">
        <h2 id="forms-title" className="flex items-baseline gap-3 text-xl font-semibold tracking-title">
          Forms of <span className="ar text-ar-sm" lang="ar" dir="rtl">{note.lemma}</span>
        </h2>
        <p className="mt-1 text-sm text-quiet">Each form appears on the card with its own audio. Changes save when you leave a field.</p>
        {note.forms.length === 0 ? (
          <p className="mt-6 text-sm text-quiet">This word has no forms yet.</p>
        ) : (
          <ul className="mt-4 divide-y divide-rule">
            {note.forms.map((f, i) => (
              <li key={f.id} className="flex flex-wrap items-center gap-x-3 gap-y-2 py-3">
                <div className="w-20 shrink-0">
                  <InlineText label={`Label of form ${i + 1}`} value={f.label} onSave={(label) => setForm(i, { label })} />
                </div>
                <div className="w-48 min-w-0 shrink">
                  <InlineText label={`Arabic of form ${i + 1}`} value={f.arabic} arabic className="text-ar-sm" onSave={(arabic) => setForm(i, { arabic })} />
                </div>
                <Clip inline note={note} slot={f.id} name={f.label} text={f.arabic} onNote={props.onNote} extra={{ label: `Remove form ${i + 1}`, icon: "removeForm", onSelect: () => save({ forms: note.forms.filter((_, j) => j !== i) }) }} />
              </li>
            ))}
          </ul>
        )}
        <div className="mt-6 flex items-center justify-between gap-3">
          <button className="action" onClick={() => save({ forms: [...note.forms, { label: "pl.", arabic: "" }] })}>Add a form</button>
          <button className="action-main" onClick={props.onClose}>Done</button>
        </div>
      </div>
    </dialog>
  );
}
