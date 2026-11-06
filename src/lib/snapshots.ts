import fs from "node:fs";
import path from "node:path";
import { audioPath } from "./audio";
import { SCHEMA_VERSION, type Store } from "./store";
import type { SnapshotInfo, SnapshotReason } from "./types";

export interface SnapshotOptions {
  now?: Date;
}

interface Manifest {
  schema: number;
  createdAt: string;
  reason: SnapshotReason;
  cards: number;
  approved: number;
  audioKeys: string[];
  unavailable: number;
}

const snapshotsDir = (root: string) => path.join(root, "snapshots");

export const snapshotAudioPath = (root: string, key: string) => audioPath(snapshotsDir(root), key);

function copyClip(from: string, to: string): void {
  if (fs.existsSync(to)) return;
  fs.mkdirSync(path.dirname(to), { recursive: true });
  fs.copyFileSync(from, to);
}

const readManifest = (root: string, id: string) =>
  JSON.parse(fs.readFileSync(path.join(snapshotsDir(root), id, "manifest.json"), "utf8")) as Manifest;

function infoOf(id: string, manifest: Manifest, dbBytes: number): SnapshotInfo {
  return {
    id,
    reason: manifest.reason,
    createdAt: manifest.createdAt,
    schema: manifest.schema,
    cards: manifest.cards,
    approved: manifest.approved,
    clips: manifest.audioKeys.length,
    unavailable: manifest.unavailable,
    dbBytes,
  };
}

export function createSnapshot(store: Store, reason: SnapshotReason, options: SnapshotOptions = {}): SnapshotInfo {
  const now = options.now ?? new Date();
  const id = `${now.toISOString().replace(/[-:]/g, "").replace(/\.\d{3}Z$/, "Z")}-${reason}`;
  const dir = path.join(snapshotsDir(store.root), id);
  const partial = path.join(snapshotsDir(store.root), `.partial-${id}`);
  fs.mkdirSync(partial, { recursive: true });
  store.exportDatabase(path.join(partial, "vocabulary.db"));
  const keys = store.audioKeys();
  for (const key of keys) copyClip(audioPath(store.root, key), snapshotAudioPath(store.root, key));
  const manifest: Manifest = {
    schema: SCHEMA_VERSION,
    createdAt: now.toISOString(),
    reason,
    ...store.counts(),
    audioKeys: keys,
    unavailable: 0,
  };
  fs.writeFileSync(path.join(partial, "manifest.json"), JSON.stringify(manifest));
  fs.renameSync(partial, dir);
  return infoOf(id, manifest, fs.statSync(path.join(dir, "vocabulary.db")).size);
}

export function restoreSnapshot(store: Store, id: string): void {
  for (const key of readManifest(store.root, id).audioKeys) {
    copyClip(snapshotAudioPath(store.root, key), audioPath(store.root, key));
  }
  const staged = path.join(store.root, "vocabulary.db.restoring");
  fs.copyFileSync(path.join(snapshotsDir(store.root), id, "vocabulary.db"), staged);
  store.close();
  fs.renameSync(staged, path.join(store.root, "vocabulary.db"));
}
