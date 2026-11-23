"use client";
import { KEYS } from "./keys";
import Modal from "./Modal";

export default function Shortcuts({ open, onClose }: { open: boolean; onClose: () => void }) {
  return (
    <Modal open={open} onClose={onClose} label="Keyboard shortcuts" className="max-w-md p-6">
      <h2 className="text-xl font-semibold tracking-title">Keyboard shortcuts</h2>
      <p className="mt-1 text-sm text-quiet">On a word, while no field has focus.</p>
      <dl className="mt-4 space-y-2">
        {KEYS.map((key) => (
          <div key={key.id} className="flex items-center justify-between gap-4">
            <dt>{key.does}</dt>
            <dd className="flex shrink-0 gap-1">
              <kbd className="keycap">Ctrl</kbd>
              <kbd className="keycap">{key.label}</kbd>
            </dd>
          </div>
        ))}
      </dl>
      <form method="dialog" className="mt-6 text-right">
        <button className="action">Close</button>
      </form>
    </Modal>
  );
}
