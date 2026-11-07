"use client";
import { useEffect, useRef } from "react";
import { missingParts, wordStatus } from "@/lib/missing";
import type { Note } from "@/lib/types";
import StatusMark from "./StatusMark";

/** The one place for what to do with a card, kept at the bottom of the window: where it stands, and how to go on. */
export default function ActionBar(props: {
  note: Note;
  problems: string[];
  leaving: boolean;
  onApprove: () => void;
}) {
  const { note } = props;
  const box = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const el = box.current;
    if (!el) return;
    const root = document.documentElement.style;
    let frame = 0;
    const watch = new ResizeObserver(() => {
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(() => root.setProperty("--bar-h", `${el.offsetHeight}px`));
    });
    watch.observe(el);
    return () => {
      cancelAnimationFrame(frame);
      watch.disconnect();
      root.removeProperty("--bar-h");
    };
  }, []);

  const approved = note.status === "approved";
  const missing = missingParts(note);
  return (
    <div ref={box} className="fixed inset-x-0 bottom-0 border-t border-rule bg-paper">
      <div className="mx-auto max-w-column px-5">
        {props.problems.length > 0 && (
          <ul role="alert" className="pt-2 text-sm text-danger">
            {props.problems.map((p) => <li key={p}>{p}</li>)}
          </ul>
        )}
        <div role="toolbar" aria-label="Card actions" className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2 py-2">
          <p className="flex min-w-0 flex-1 items-center gap-2 text-sm text-quiet">
            <StatusMark status={wordStatus(note)} />
            {!approved && <span>{missing.length > 0 ? `Still needs ${missing.join(", ")}` : "Ready to approve"}</span>}
          </p>
          <div className="grid w-full grid-cols-2 gap-3 sm:flex sm:w-auto">
            {approved ? (
              <a href="/next" className="action-main col-start-2 w-full sm:w-44">Next</a>
            ) : (
              <button className="action-main col-start-2 w-full sm:w-44" disabled={props.leaving} onClick={props.onApprove}>Approve</button>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
