"use client";
import { useId, useRef, useState } from "react";
import Backups from "./Backups";
import Shortcuts from "./Shortcuts";
import UnusedAudio from "./UnusedAudio";

type Open = "backups" | "audio" | "shortcuts" | null;

export default function Menu() {
  const id = useId();
  const anchor = `--${id.replace(/[^a-z0-9]/gi, "")}`;
  const trigger = useRef<HTMLButtonElement>(null);
  const [open, setOpen] = useState<Open>(null);

  const close = () => {
    setOpen(null);
    trigger.current?.focus();
  };

  return (
    <>
      <button ref={trigger} className="action m-1 min-w-control-sm px-2" aria-label="Menu" popoverTarget={id} style={{ anchorName: anchor }}>
        <svg viewBox="0 0 16 4" width="16" height="4" aria-hidden="true" fill="currentColor">
          <circle cx="2" cy="2" r="1.6" /><circle cx="8" cy="2" r="1.6" /><circle cx="14" cy="2" r="1.6" />
        </svg>
      </button>
      <div id={id} popover="auto" className="options options-end" style={{ positionAnchor: anchor }}>
        <button className="options-item" popoverTarget={id} popoverTargetAction="hide" onClick={() => setOpen("backups")}>
          Backups
        </button>
        <button className="options-item" popoverTarget={id} popoverTargetAction="hide" onClick={() => setOpen("audio")}>
          Prune audio
        </button>
        <button className="options-item" popoverTarget={id} popoverTargetAction="hide" onClick={() => setOpen("shortcuts")}>
          Keyboard shortcuts
        </button>
      </div>
      <Backups open={open === "backups"} onClose={close} />
      <UnusedAudio open={open === "audio"} onClose={close} />
      <Shortcuts open={open === "shortcuts"} onClose={close} />
    </>
  );
}
