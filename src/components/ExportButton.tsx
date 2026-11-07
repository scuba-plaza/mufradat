"use client";
import { useRef, useState } from "react";
import { refusal } from "./api";

export default function ExportButton({ q, count }: { q: string; count: number }) {
  const [message, setMessage] = useState("");
  const building = useRef(false);

  async function exportDeck() {
    if (building.current) return;
    building.current = true;
    setMessage("Preparing the deck…");
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
    }
  }

  return (
    <>
      <button
        type="button"
        className="action min-h-control min-w-control px-0"
        title={`Export ${count} approved ${count === 1 ? "card" : "cards"}`}
        disabled={count === 0}
        onClick={exportDeck}
      >
        <svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true" focusable="false" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" /><path d="M7 10l5 5 5-5" /><path d="M12 15V3" />
        </svg>
      </button>
      <p role="status" className="w-full break-words px-1 text-sm text-quiet empty:hidden">{message}</p>
    </>
  );
}
