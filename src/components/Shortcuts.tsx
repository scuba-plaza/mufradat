"use client";
import { useRef } from "react";
import { KEYS } from "./keys";

export default function Shortcuts() {
  const dialog = useRef<HTMLDialogElement>(null);

  return (
    <>
      <button className="action-quiet min-w-control-sm" aria-label="Keyboard shortcuts" onClick={() => dialog.current?.showModal()}>?</button>
      <dialog
        ref={dialog}
        aria-label="Keyboard shortcuts"
        className="w-[calc(100%-2.5rem)] max-w-md rounded-modal border border-rule bg-surface p-6 text-ink shadow-modal backdrop:bg-black/70"
        onClick={(e) => e.target === dialog.current && dialog.current.close()}
      >
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
      </dialog>
    </>
  );
}
