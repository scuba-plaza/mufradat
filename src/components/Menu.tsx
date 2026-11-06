"use client";
import { useId, useState } from "react";
import Shortcuts from "./Shortcuts";

type Open = "shortcuts" | null;

export default function Menu() {
  const id = useId();
  const anchor = `--${id.replace(/[^a-z0-9]/gi, "")}`;
  const [open, setOpen] = useState<Open>(null);

  return (
    <>
      <button className="action min-w-control-sm px-2" aria-label="Menu" popoverTarget={id} style={{ anchorName: anchor }}>
        <svg viewBox="0 0 16 4" width="16" height="4" aria-hidden="true" fill="currentColor">
          <circle cx="2" cy="2" r="1.6" /><circle cx="8" cy="2" r="1.6" /><circle cx="14" cy="2" r="1.6" />
        </svg>
      </button>
      <div id={id} popover="auto" className="options options-end" style={{ positionAnchor: anchor }}>
        <button className="options-item" popoverTarget={id} popoverTargetAction="hide" onClick={() => setOpen("shortcuts")}>
          Keyboard shortcuts
        </button>
      </div>
      <Shortcuts open={open === "shortcuts"} onClose={() => setOpen(null)} />
    </>
  );
}
