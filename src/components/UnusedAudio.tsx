"use client";
import { useEffect, useState } from "react";
import { api } from "./api";
import { plural, size } from "./format";
import Modal from "./Modal";

interface Totals {
  files: number;
  bytes: number;
}

type Phase =
  | { kind: "ask"; found: Totals }
  | { kind: "none" }
  | { kind: "done"; deleted: Totals }
  | { kind: "failed"; message: string };

function sentence(phase: Phase | null): string {
  switch (phase?.kind) {
    case "ask":
      return `Delete ${phase.found.files} unused audio ${plural(phase.found.files, "file")} (${size(phase.found.bytes)})?`;
    case "none":
      return "No unused audio files.";
    case "done":
      return `Deleted ${phase.deleted.files} audio ${plural(phase.deleted.files, "file")} (${size(phase.deleted.bytes)}).`;
    default:
      return "";
  }
}

export default function UnusedAudio({ open, onClose }: { open: boolean; onClose: () => void }) {
  const [phase, setPhase] = useState<Phase | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (open) check();
  }, [open]);

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

  const close = () => {
    setPhase(null);
    onClose();
  };

  const remove = () => run(async () => ({ kind: "done", deleted: await api<Totals>("/api/audio/unused", { method: "DELETE" }) }));

  return (
    <Modal open={phase !== null} onClose={close} labelledBy="unused-audio-title" className="text-sm max-w-lg p-0">
      <div className="p-6">
        <h2 id="unused-audio-title" className="text-xl font-semibold tracking-title">Unused audio files</h2>
        <p role="status" className="mt-2 empty:hidden">{sentence(phase)}</p>
        {phase?.kind === "failed" && <p role="alert" className="mt-2 break-words text-danger">{phase.message}</p>}
        {phase?.kind === "ask" && <p className="mt-2 text-sm text-quiet">Cards keep every clip they use.</p>}
        <div className="mt-6 flex justify-end gap-3">
          {phase?.kind === "ask" ? (
            <>
              <button className="action" onClick={close}>Cancel</button>
              <button className="action-danger" disabled={busy} onClick={remove}>Delete</button>
            </>
          ) : (
            <button className="action" onClick={close}>Close</button>
          )}
        </div>
      </div>
    </Modal>
  );
}
