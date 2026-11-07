"use client";
import { useId, useRef, useState } from "react";
import Backups from "./Backups";
import { plural } from "./format";
import Shortcuts from "./Shortcuts";
import UnusedAudio from "./UnusedAudio";

type Open = "backups" | "audio" | "shortcuts" | null;

export default function Menu({ approved }: { approved: number }) {
  const id = useId();
  const anchor = `--${id.replace(/[^a-z0-9]/gi, "")}`;
  const trigger = useRef<HTMLButtonElement>(null);
  const [open, setOpen] = useState<Open>(null);
  const [message, setMessage] = useState("");

  const close = () => {
    setOpen(null);
    trigger.current?.focus();
  };

  async function exportDeck() {
    setMessage("Preparing the deck…");
    try {
      const res = await fetch("/api/export");
      if (!res.ok) throw new Error(((await res.json()) as { error: string }).error);
      const cards = Number(res.headers.get("X-Card-Count"));
      const link = document.createElement("a");
      link.href = URL.createObjectURL(await res.blob());
      link.download = /filename="([^"]+)"/.exec(res.headers.get("Content-Disposition") ?? "")?.[1] ?? "arabic-deck.apkg";
      link.click();
      setTimeout(() => URL.revokeObjectURL(link.href), 10_000);
      setMessage(`Exported ${cards} ${plural(cards, "card")}.`);
    } catch (e) {
      setMessage((e as Error).message);
    }
  }

  return (
    <>
      <button ref={trigger} className="action m-1 min-w-control-sm px-2" aria-label="Menu" popoverTarget={id} style={{ anchorName: anchor }}>
        <svg viewBox="0 0 16 4" width="16" height="4" aria-hidden="true" fill="currentColor">
          <circle cx="2" cy="2" r="1.6" /><circle cx="8" cy="2" r="1.6" /><circle cx="14" cy="2" r="1.6" />
        </svg>
      </button>
      <div id={id} popover="auto" className="options options-end" style={{ positionAnchor: anchor }}>
        <button className="options-item" disabled={approved === 0} popoverTarget={id} popoverTargetAction="hide" onClick={exportDeck}>
          Export
        </button>
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
      <p role="status" className="w-full break-words pb-2 text-sm text-quiet empty:hidden">{message}</p>
    </>
  );
}
