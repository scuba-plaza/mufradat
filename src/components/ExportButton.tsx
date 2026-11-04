"use client";
import { useState } from "react";
import { post } from "./api";

/** Writes every approved card to an Anki package and downloads it. */
export default function ExportButton({ approved }: { approved: number }) {
  const [message, setMessage] = useState("");
  async function run() {
    setMessage("");
    try {
      const r = await post<{ file: string; path: string; cards: number }>("/api/export", {});
      setMessage(`Exported ${r.cards} cards to ${r.path}`);
      window.location.href = `/api/export?file=${r.file}`;
    } catch (e) {
      setMessage((e as Error).message);
    }
  }
  return (
    <div className="py-6 text-sm">
      <button className="action" disabled={approved === 0} onClick={run}>
        {approved === 0 ? "Export to Anki (nothing approved yet)" : `Export ${approved} approved ${approved === 1 ? "card" : "cards"} to Anki`}
      </button>
      {message && <p role="status" className="mt-1 break-words px-1 text-quiet">{message}</p>}
    </div>
  );
}
