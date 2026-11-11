"use client";
import { useEffect, useRef } from "react";
import type { Note } from "@/lib/types";

export default function SuspendDialog(props: { note: Note; open: boolean; onClose: () => void; onSuspend: () => void }) {
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
      aria-labelledby="suspend-title"
      onClose={props.onClose}
      onClick={(e) => e.target === dialog.current && props.onClose()}
      className="w-[calc(100%-2.5rem)] max-w-lg rounded-modal border border-rule bg-surface p-0 text-ink shadow-modal backdrop:bg-black/70"
    >
      <div className="p-6">
        <h2 id="suspend-title" className="text-xl font-semibold tracking-title">Suspend this word?</h2>
        <p className="mt-2 text-sm text-quiet">
          The card of <span className="ar" lang="ar" dir="rtl">{props.note.lemma}</span> takes its approval back, is
          never exported, and is found only by the search status:suspended.
        </p>
        <div className="mt-6 flex justify-end gap-3">
          <button className="action" onClick={props.onClose}>Cancel</button>
          <button className="action-danger" onClick={props.onSuspend}>Suspend</button>
        </div>
      </div>
    </dialog>
  );
}
