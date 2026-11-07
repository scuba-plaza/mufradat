"use client";
import { useEffect, useRef } from "react";
import type { Note } from "@/lib/types";

export default function ResetDialog(props: { note: Note; open: boolean; busy: boolean; onClose: () => void; onReset: () => void }) {
  const { note } = props;
  const dialog = useRef<HTMLDialogElement>(null);

  useEffect(() => {
    const el = dialog.current;
    if (!el) return;
    if (props.open && !el.open) el.showModal();
    if (!props.open && el.open) el.close();
  }, [props.open]);

  return (
    <dialog
      ref={dialog}
      aria-labelledby="reset-title"
      onClose={props.onClose}
      onClick={(e) => e.target === dialog.current && props.onClose()}
      className="w-[calc(100%-2.5rem)] max-w-lg rounded-modal border border-rule bg-surface p-0 text-ink shadow-modal backdrop:bg-black/70"
    >
      <div className="p-6">
        <h2 id="reset-title" className="text-xl font-semibold tracking-title">Reset this word?</h2>
        <p className="mt-2 text-sm text-quiet">
          The card is deleted with its example, forms and audio clips.{" "}
          {note.rank > 0 ? "The word goes back to Not started." : "The word leaves your list; search for it to add it again."}
          {note.status === "approved" && " It is no longer part of the Anki export."}
        </p>
        <div className="mt-6 flex justify-end gap-3">
          <button className="action" onClick={props.onClose}>Cancel</button>
          <button className="action-danger" disabled={props.busy} onClick={props.onReset}>Reset</button>
        </div>
      </div>
    </dialog>
  );
}
