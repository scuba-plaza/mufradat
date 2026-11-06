"use client";
import { useEffect, useRef, useState } from "react";
import { api } from "./api";
import { plural, size } from "./format";

interface Totals {
  files: number;
  bytes: number;
}

type Phase =
  | { kind: "ask"; found: Totals }
  | { kind: "none" }
  | { kind: "done"; deleted: Totals }
  | { kind: "failed"; message: string };

export default function UnusedAudio() {
  const dialog = useRef<HTMLDialogElement>(null);
  const [phase, setPhase] = useState<Phase | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    const el = dialog.current;
    if (!el) return;
    if (phase && !el.open) el.showModal();
    if (!phase && el.open) el.close();
  }, [phase]);

  async function run(call: () => Promise<Phase>) {
    setBusy(true);
    try {
      setPhase(await call());
    } catch (e) {
      setPhase({ kind: "failed", message: (e as Error).message });
    } finally {
      setBusy(false);
    }
  }

  const check = () =>
    run(async () => {
      const found = await api<Totals>("/api/audio/unused");
      return found.files === 0 ? { kind: "none" } : { kind: "ask", found };
    });

  const remove = () => run(async () => ({ kind: "done", deleted: await api<Totals>("/api/audio/unused", { method: "DELETE" }) }));

  const sentence =
    phase?.kind === "ask"
      ? `Delete ${phase.found.files} unused audio ${plural(phase.found.files, "file")} (${size(phase.found.bytes)})?`
      : phase?.kind === "none"
        ? "No unused audio files."
        : phase?.kind === "done"
          ? `Deleted ${phase.deleted.files} audio ${plural(phase.deleted.files, "file")} (${size(phase.deleted.bytes)}).`
          : "";

  return (
    <div className="text-sm">
      <button className="action-quiet" aria-haspopup="dialog" disabled={busy} onClick={check}>Delete unused audio…</button>
      <dialog
        ref={dialog}
        aria-labelledby="unused-audio-title"
        onClose={() => setPhase(null)}
        onClick={(e) => e.target === dialog.current && setPhase(null)}
        className="w-[calc(100%-2.5rem)] max-w-lg rounded-modal border border-rule bg-surface p-0 text-ink shadow-modal backdrop:bg-black/70"
      >
        <div className="p-6">
          <h2 id="unused-audio-title" className="text-xl font-semibold tracking-title">Unused audio files</h2>
          <p role="status" className="mt-2 empty:hidden">{sentence}</p>
          {phase?.kind === "failed" && <p role="alert" className="mt-2 break-words text-danger">{phase.message}</p>}
          {phase?.kind === "ask" && <p className="mt-2 text-sm text-quiet">Cards keep every clip they use.</p>}
          <div className="mt-6 flex justify-end gap-3">
            {phase?.kind === "ask" ? (
              <>
                <button className="action" onClick={() => setPhase(null)}>Cancel</button>
                <button className="action-main" disabled={busy} onClick={remove}>Delete</button>
              </>
            ) : (
              <button className="action" onClick={() => setPhase(null)}>Close</button>
            )}
          </div>
        </div>
      </dialog>
    </div>
  );
}
