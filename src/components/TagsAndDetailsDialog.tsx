"use client";
import { useEffect, useRef } from "react";
import type { Note } from "@/lib/types";
import Details from "./Details";
import Tags from "./Tags";

export default function TagsAndDetailsDialog(props: {
  note: Note;
  open: boolean;
  onClose: () => void;
  save: (patch: Record<string, unknown>) => void;
  saveTags: (change: (tags: string[]) => string[]) => Promise<boolean>;
  tagProblem: string;
  clearTagProblem: () => void;
  problems: string[];
}) {
  const { note } = props;
  const dialog = useRef<HTMLDialogElement>(null);
  const input = useRef<HTMLInputElement>(null);

  useEffect(() => {
    const el = dialog.current;
    if (!el) return;
    if (props.open && !el.open) {
      el.showModal();
      input.current?.focus();
    }
    if (!props.open && el.open) el.close();
  }, [props.open]);

  return (
    <dialog
      ref={dialog}
      aria-labelledby="tags-and-details-title"
      onClose={props.onClose}
      onClick={(e) => e.target === dialog.current && props.onClose()}
      className="max-h-[90vh] w-[calc(100%-2.5rem)] max-w-lg overflow-y-auto rounded-modal border border-rule bg-surface p-0 text-ink shadow-modal backdrop:bg-black/70"
    >
      <div className="p-6">
        <h2 id="tags-and-details-title" className="flex items-baseline gap-3 text-xl font-semibold tracking-title">
          Tags and details of <span className="ar text-ar-sm" lang="ar" dir="rtl">{note.lemma}</span>
        </h2>
        <section aria-labelledby="tags-heading" className="mt-6">
          <h3 id="tags-heading" className="text-sm font-medium text-quiet">Tags</h3>
          <div className="mt-3">
            <Tags key={String(props.open)} tags={note.tags} problem={props.tagProblem} inputRef={input} onChange={props.saveTags} onEdit={props.clearTagProblem} />
          </div>
        </section>
        <section aria-labelledby="details-heading" className="mt-6">
          <h3 id="details-heading" className="text-sm font-medium text-quiet">Details</h3>
          <div className="mt-3">
            <Details note={note} save={props.save} />
          </div>

        </section>
        <div className="mt-6 flex items-start justify-between gap-3">
          <div className="min-w-0 flex-1 break-words text-sm text-danger">
            {props.tagProblem && <p role="alert">{props.tagProblem}</p>}
            {props.problems.length > 0 && (
              <ul role="alert">
                {props.problems.map((p) => <li key={p}>{p}</li>)}
              </ul>
            )}
          </div>
          <button className="action-main" onClick={props.onClose}>Done</button>
        </div>
      </div>
    </dialog>
  );
}
