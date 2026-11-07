"use client";
import { useEffect, useLayoutEffect, useRef, useState } from "react";
import type { SnapshotInfo, SnapshotReason, SnapshotUsage, WorkspaceCheck } from "@/lib/types";
import { api } from "./api";
import { plural, size } from "./format";
import Spinner from "./Spinner";

interface BackupsData {
  snapshots: SnapshotInfo[];
  usage: SnapshotUsage;
  check: WorkspaceCheck;
  lastDownloadedAt: string | null;
}

const REASONS: Record<SnapshotReason, string> = {
  manual: "Backed up by you",
  daily: "Daily",
  "before-reset": "Before a reset",
  "before-restore": "Before a restore",
  imported: "Imported",
};

const when = (s: SnapshotInfo) => new Date(s.createdAt).toLocaleString();

interface Confirming {
  snapshot: SnapshotInfo;
  action: "restore" | "delete";
}

export default function Backups({ open, onClose }: { open: boolean; onClose: () => void }) {
  const dialog = useRef<HTMLDialogElement>(null);
  const picker = useRef<HTMLInputElement>(null);
  const list = useRef<HTMLOListElement>(null);
  const [floor, setFloor] = useState(0);
  const [data, setData] = useState<BackupsData | null>(null);
  const [confirming, setConfirming] = useState<Confirming | null>(null);
  const [busy, setBusy] = useState(false);
  const [importing, setImporting] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    const el = dialog.current;
    if (!el) return;
    if (open && !el.open) el.showModal();
    if (!open && el.open) el.close();
  }, [open]);

  useEffect(() => {
    if (open) run(load);
  }, [open]);

  useLayoutEffect(() => {
    const el = list.current;
    if (!el) return;
    const measure = () => setFloor(Array.from(el.children).slice(0, 3).reduce((sum, row) => sum + row.getBoundingClientRect().height, 0));
    measure();
    const watch = new ResizeObserver(measure);
    watch.observe(el);
    return () => watch.disconnect();
  }, [data, confirming]);

  async function run(call: () => Promise<void>) {
    setBusy(true);
    setError("");
    try {
      await call();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  const load = async () => setData(await api<BackupsData>("/api/backups"));

  const backUp = () =>
    run(async () => {
      await api("/api/backups", { method: "POST" });
      await load();
    });

  const importFile = (file: File) =>
    run(async () => {
      setImporting(true);
      try {
        await api("/api/backups/import", { method: "POST", body: file });
        await load();
      } finally {
        setImporting(false);
      }
    });

  const restore = (s: SnapshotInfo) =>
    run(async () => {
      setConfirming(null);
      await api(`/api/backups/${s.id}/restore`, { method: "POST" });
      window.location.assign("/");
    });

  const remove = (s: SnapshotInfo) =>
    run(async () => {
      setConfirming(null);
      await api(`/api/backups/${s.id}`, { method: "DELETE" });
      await load();
    });

  const close = () => {
    setData(null);
    setConfirming(null);
    setError("");
    onClose();
  };

  return (
    <dialog
      ref={dialog}
      aria-labelledby="backups-title"
      onClose={close}
      onClick={(e) => e.target === dialog.current && close()}
      className="text-sm open:flex open:flex-col w-[calc(100%-2.5rem)] max-w-2xl rounded-modal border border-rule bg-surface p-0 text-ink shadow-modal backdrop:bg-black/70"
    >
      <div className="flex min-h-0 flex-col p-6">
        <h2 id="backups-title" className="text-xl font-semibold tracking-title">Backups</h2>
        <p className="mt-2 text-sm text-quiet">
          These backups are on this computer. Download one to keep a copy somewhere else.
          {data && ` Last downloaded: ${data.lastDownloadedAt ? new Date(data.lastDownloadedAt).toLocaleString() : "never"}.`}
        </p>
        {error && <p role="alert" className="mt-2 break-words text-danger">{error}</p>}
        {data && (
          <>
            <p className="mt-2 text-sm text-quiet">
              Automatic backups use {size(data.usage.automaticBytes)} of {size(data.usage.budgetBytes)}; all backups use {size(data.usage.totalBytes)}.
              {data.usage.automaticBytes > data.usage.budgetBytes && " The newest automatic backups alone are over that limit, and they are still kept."}
            </p>
            {(data.check.integrity !== "ok" || data.check.missingAudio.length > 0) && (
              <ul className="mt-3 space-y-1 text-danger">
                {data.check.integrity !== "ok" && <li className="break-words">The database reports a problem: {data.check.integrity} Restore a backup from before this appeared.</li>}
                {data.check.missingAudio.map((m) => (
                  <li key={`${m.noteId}-${m.slot}`}>
                    No audio file for the {m.slotName} of <a className="underline" href={`/cards/${m.noteId}`}>{m.lemma}</a>. Open the card to generate or upload it again.
                  </li>
                ))}
              </ul>
            )}
            {confirming ? (
              <div className="mt-4">
                <p role="status">
                  {confirming.action === "restore"
                    ? `Restore the backup from ${when(confirming.snapshot)}? It has ${confirming.snapshot.cards} ${plural(confirming.snapshot.cards, "card")} (${confirming.snapshot.approved} approved); the workspace has ${data.check.cards} (${data.check.approved} approved) now. Your current state is saved first.`
                    : `Delete the backup from ${when(confirming.snapshot)}? It cannot be brought back.`}
                </p>
                <div className="mt-4 flex justify-end gap-3">
                  <button className="action" autoFocus onClick={() => setConfirming(null)}>Cancel</button>
                  <button
                    className="action-danger"
                    disabled={busy}
                    onClick={() => (confirming.action === "restore" ? restore(confirming.snapshot) : remove(confirming.snapshot))}
                  >
                    {confirming.action === "restore" ? "Restore" : "Delete"}
                  </button>
                </div>
              </div>
            ) : (
              <>
                <div className="mt-4 flex flex-wrap gap-2">
                  <button className="action" disabled={busy} onClick={backUp}>Back up now</button>
                  <button className="action relative" disabled={busy} aria-busy={importing || undefined} onClick={() => picker.current?.click()}>
                    <span className={importing ? "invisible" : ""}>Import backup…</span>
                    {importing && <span className="absolute inset-0 flex items-center justify-center"><Spinner size={16} /></span>}
                  </button>
                  <input
                    ref={picker}
                    type="file"
                    accept=".zip,application/zip"
                    hidden
                    onChange={(e) => {
                      const file = e.target.files?.[0];
                      e.target.value = "";
                      if (file) importFile(file);
                    }}
                  />
                </div>
                <p role="status" className="sr-only">{importing ? "Importing…" : ""}</p>
                {data.snapshots.length === 0 ? (
                  <p className="mt-4">No backups yet.</p>
                ) : (
                  <ol ref={list} aria-label="Backups" className="mt-4 overflow-y-auto" style={{ minHeight: floor }}>
                    {data.snapshots.map((s) => (
                      <li key={s.id} className="border-t border-rule py-3">
                        <p>{REASONS[s.reason]} · {when(s)}</p>
                        <p className="text-quiet">{s.cards} {plural(s.cards, "card")} · {s.approved} approved</p>
                        <div className="mt-1 flex flex-wrap gap-1">
                          <button
                            className="action-quiet"
                            aria-label={`Restore the backup from ${when(s)}`}
                            disabled={busy}
                            onClick={() => setConfirming({ snapshot: s, action: "restore" })}
                          >
                            Restore…
                          </button>
                          <a className="action-quiet" href={`/api/backups/${s.id}`} download aria-label={`Download the backup from ${when(s)}`}>
                            Download
                          </a>
                          <button
                            className="action-quiet"
                            aria-label={`Delete the backup from ${when(s)}`}
                            disabled={busy}
                            onClick={() => setConfirming({ snapshot: s, action: "delete" })}
                          >
                            Delete…
                          </button>
                        </div>
                      </li>
                    ))}
                  </ol>
                )}
              </>
            )}
          </>
        )}
        {!confirming && (
          <div className="mt-6 flex justify-end gap-3">
            <button className="action" onClick={close}>Close</button>
          </div>
        )}
      </div>
    </dialog>
  );
}
