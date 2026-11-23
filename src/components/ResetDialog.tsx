"use client";
import type { Note } from "@/lib/types";
import Modal from "./Modal";

export default function ResetDialog(props: { note: Note; open: boolean; busy: boolean; onClose: () => void; onReset: () => void }) {
  const { note } = props;
  return (
    <Modal open={props.open} onClose={props.onClose} labelledBy="reset-title" className="max-w-lg p-0">
      <div className="p-6">
        <h2 id="reset-title" className="text-xl font-semibold tracking-title">Reset this word?</h2>
        <p className="mt-2 text-sm text-quiet">
          The card is deleted with its example, forms, tags and audio clips.{" "}
          {note.rank > 0 ? "The word goes back to Not started." : "The word leaves your list; search for it to add it again."}
          {note.status === "approved" && " It is no longer part of the Anki export."}
        </p>
        <div className="mt-6 flex justify-end gap-3">
          <button className="action" onClick={props.onClose}>Cancel</button>
          <button className="action-danger" disabled={props.busy} onClick={props.onReset}>Reset</button>
        </div>
      </div>
    </Modal>
  );
}
