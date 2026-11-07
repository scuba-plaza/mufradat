"use client";
import { useEffect, useId, useRef, useState } from "react";
import { refusal } from "./api";
import Spinner from "./Spinner";

export default function ExportButton({ q, count }: { q: string; count: number }) {
  const [message, setMessage] = useState("");
  const [preparing, setPreparing] = useState(false);
  const building = useRef(false);
  const notice = useRef<HTMLDivElement>(null);
  const id = useId();
  const anchor = `--${id.replace(/[^a-z0-9]/gi, "")}`;

  useEffect(() => {
    const el = notice.current;
    if (!el) return;
    if (message && !el.matches(":popover-open")) el.showPopover();
    if (!message && el.matches(":popover-open")) el.hidePopover();
  }, [message]);

  async function exportDeck() {
    if (building.current) return;
    building.current = true;
    setPreparing(true);
    setMessage("");
    try {
      const res = await fetch(q.trim() === "" ? "/api/export" : `/api/export?q=${encodeURIComponent(q)}`);
      if (!res.ok) throw await refusal(res);
      const link = document.createElement("a");
      link.href = URL.createObjectURL(await res.blob());
      link.download = /filename="([^"]+)"/.exec(res.headers.get("Content-Disposition") ?? "")?.[1] ?? "arabic-deck.apkg";
      link.click();
      setTimeout(() => URL.revokeObjectURL(link.href), 10_000);
      setMessage("");
    } catch (e) {
      setMessage((e as Error).message);
    } finally {
      building.current = false;
      setPreparing(false);
    }
  }

  return (
    <>
      <button
        type="button"
        className="action min-h-control min-w-control px-0"
        title={`Export ${count} approved ${count === 1 ? "card" : "cards"}`}
        disabled={count === 0}
        aria-busy={preparing || undefined}
        style={{ anchorName: anchor }}
        onClick={exportDeck}
      >
        {preparing ? (
          <Spinner size={18} />
        ) : (
          <svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true" focusable="false" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" /><path d="M7 10l5 5 5-5" /><path d="M12 15V3" />
          </svg>
        )}
      </button>
      <div
        ref={notice}
        popover="auto"
        role="alert"
        className="options options-end max-w-[min(28rem,calc(100vw-2.5rem))] break-words px-3 py-2 text-sm text-danger"
        style={{ positionAnchor: anchor }}
        onToggle={(e) => e.newState === "closed" && setMessage("")}
      >
        {message}
      </div>
      <p role="status" className="sr-only">{preparing ? "Preparing the deck…" : ""}</p>
    </>
  );
}
