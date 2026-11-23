"use client";
import type { Note } from "@/lib/types";
import Modal from "./Modal";

export default function SuspendDialog(props: { note: Note; open: boolean; onClose: () => void; onSuspend: () => void }) {
  return (
    <Modal open={props.open} onClose={props.onClose} labelledBy="suspend-title" className="max-w-lg p-0">
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
    </Modal>
  );
}
