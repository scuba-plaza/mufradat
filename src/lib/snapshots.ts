import Database from "better-sqlite3";
import fs from "node:fs";
import path from "node:path";
import { audioKey, audioPath } from "./audio";
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

const ID_PATTERN = /^\d{8}T\d{6}Z-(manual|daily|before-reset|before-restore|imported)(-\d+)?$/;

export const isSnapshotId = (id: string): boolean => ID_PATTERN.test(id);

function readManifest(root: string, id: string): Manifest | null {
  try {
    return JSON.parse(fs.readFileSync(path.join(snapshotsDir(root), id, "manifest.json"), "utf8")) as Manifest;
  } catch {
    return null;
  }
}

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

function databaseIsSound(file: string): boolean {
  try {
    const db = new Database(file, { readonly: true, fileMustExist: true });
    try {
      return db.pragma("integrity_check", { simple: true }) === "ok";
    } finally {
      db.close();
    }
  } catch {
    return false;
  }
}

function clipIsSound(file: string, key: string): boolean {
  return fs.existsSync(file) && audioKey(fs.readFileSync(file)) === key;
}

function verifiedManifest(root: string, id: string): Manifest {
  const manifest = isSnapshotId(id) ? readManifest(root, id) : null;
  if (!manifest) throw new Error("There is no such backup.");
  if (manifest.schema > SCHEMA_VERSION) throw new Error("This backup was written by a newer Mufradat.");
  if (!databaseIsSound(path.join(snapshotsDir(root), id, "vocabulary.db"))) throw new Error("The backup's database is damaged.");
  if (!manifest.audioKeys.every((key) => clipIsSound(snapshotAudioPath(root, key), key))) {
    throw new Error("The backup is missing or has damaged audio clips.");
  }
  return manifest;
}

export function listSnapshots(root: string): SnapshotInfo[] {
  return fs
    .readdirSync(snapshotsDir(root), { withFileTypes: true })
    .filter((entry) => entry.isDirectory() && isSnapshotId(entry.name))
    .flatMap((entry) => {
      const manifest = readManifest(root, entry.name);
      if (!manifest) return [];
      return [infoOf(entry.name, manifest, fs.statSync(path.join(snapshotsDir(root), entry.name, "vocabulary.db")).size)];
    })
    .sort((a, b) => (a.id < b.id ? 1 : -1));
}

export function restoreSnapshot(store: Store, id: string, options: SnapshotOptions = {}): void {
  const manifest = verifiedManifest(store.root, id);
  createSnapshot(store, "before-restore", options);
  for (const key of manifest.audioKeys) {
    copyClip(snapshotAudioPath(store.root, key), audioPath(store.root, key));
  }
  const staged = path.join(store.root, "vocabulary.db.restoring");
  fs.copyFileSync(path.join(snapshotsDir(store.root), id, "vocabulary.db"), staged);
  store.close();
  fs.renameSync(staged, path.join(store.root, "vocabulary.db"));
}
