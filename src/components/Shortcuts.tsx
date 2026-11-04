"use client";
import { useEffect, useRef } from "react";
import { keyIsShortcut } from "./typing";

const GROUPS = [
  {
    title: "Anywhere",
    keys: [
      { keys: ["/"], does: "Search" },
      { keys: ["?"], does: "Show this list" },
    ],
  },
  {
    title: "On a word",
    keys: [
      { keys: ["Ctrl", "Enter"], does: "Approve and go to the next word" },
      { keys: ["g"], does: "Generate an example" },
      { keys: ["w"], does: "Play the word" },
      { keys: ["e"], does: "Play the example" },
      { keys: ["d"], does: "Show or hide the details" },
      { keys: ["s"], does: "Skip this word" },
    ],
  },
];

/** The header's help button, the list of shortcuts it opens, and the two shortcuts that work on every page. */
export default function Shortcuts() {
  const dialog = useRef<HTMLDialogElement>(null);

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") return dialog.current?.close();
      if (!keyIsShortcut(e)) return;
      if (e.key === "?") {
        e.preventDefault();
        dialog.current?.showModal();
      } else if (e.key === "/") {
        e.preventDefault();
        window.location.assign("/search");
      }
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  return (
    <>
      <button className="action" aria-label="Keyboard shortcuts" onClick={() => dialog.current?.showModal()}>?</button>
      <dialog
        ref={dialog}
        aria-label="Keyboard shortcuts"
        className="w-[calc(100%-2.5rem)] max-w-md rounded-lg border border-rule bg-paper p-6 text-ink backdrop:bg-ink/40"
        onClick={(e) => e.target === dialog.current && dialog.current.close()}
      >
        <h2 className="font-serif text-xl">Keyboard shortcuts</h2>
        {GROUPS.map((group) => (
          <section key={group.title} className="mt-5">
            <h3 className="text-sm text-quiet">{group.title}</h3>
            <dl className="mt-2 space-y-2">
              {group.keys.map((row) => (
                <div key={row.does} className="flex items-center justify-between gap-4">
                  <dt>{row.does}</dt>
                  <dd className="flex shrink-0 gap-1">
                    {row.keys.map((key) => <kbd key={key} className="keycap">{key}</kbd>)}
                  </dd>
                </div>
              ))}
            </dl>
          </section>
        ))}
        <form method="dialog" className="mt-6 text-right">
          <button className="action">Close</button>
        </form>
      </dialog>
    </>
  );
}
