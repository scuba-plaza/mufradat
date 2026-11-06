import Database from "better-sqlite3";
import fs from "node:fs";
import path from "node:path";
import { audioKey, audioPath } from "./audio";
import { SCHEMA_VERSION, type Store } from "./store";
import { SNAPSHOT_REASONS, type SnapshotInfo, type SnapshotReason } from "./types";

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

function copyClip(from: string, to: string, key: string): boolean {
  if (fs.existsSync(to)) return true;
  if (!fs.existsSync(from)) return false;
  const bytes = fs.readFileSync(from);
  if (audioKey(bytes) !== key) return false;
  fs.mkdirSync(path.dirname(to), { recursive: true });
  const staged = `${to}.tmp`;
  fs.writeFileSync(staged, bytes);
  fs.renameSync(staged, to);
  return true;
}

const ID_PATTERN = new RegExp(`^\\d{8}T\\d{6}Z-(${SNAPSHOT_REASONS.join("|")})(-\\d+)?$`);

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

function freeId(root: string, now: Date, reason: SnapshotReason): string {
  const stamp = now.toISOString().replace(/[-:]/g, "").replace(/\.\d{3}Z$/, "Z");
  const taken = (id: string) =>
    fs.existsSync(path.join(snapshotsDir(root), id)) || fs.existsSync(path.join(snapshotsDir(root), `.partial-${id}`));
  let id = `${stamp}-${reason}`;
  for (let n = 2; taken(id); n++) id = `${stamp}-${reason}-${n}`;
  return id;
}

export function createSnapshot(store: Store, reason: SnapshotReason, options: SnapshotOptions = {}): SnapshotInfo {
  const now = options.now ?? new Date();
  const id = freeId(store.root, now, reason);
  const dir = path.join(snapshotsDir(store.root), id);
  const partial = path.join(snapshotsDir(store.root), `.partial-${id}`);
  fs.mkdirSync(partial, { recursive: true });
  try {
    store.exportDatabase(path.join(partial, "vocabulary.db"));
    const keys = store.audioKeys();
    const present = keys.filter((key) => copyClip(audioPath(store.root, key), snapshotAudioPath(store.root, key), key));
    const manifest: Manifest = {
      schema: SCHEMA_VERSION,
      createdAt: now.toISOString(),
      reason,
      ...store.counts(),
      audioKeys: present,
      unavailable: keys.length - present.length,
    };
    fs.writeFileSync(path.join(partial, "manifest.json"), JSON.stringify(manifest));
    fs.renameSync(partial, dir);
    return infoOf(id, manifest, fs.statSync(path.join(dir, "vocabulary.db")).size);
  } catch (error) {
    fs.rmSync(partial, { recursive: true, force: true });
    throw error;
  }
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

function fileBytes(file: string): number | null {
  try {
    return fs.statSync(file).size;
  } catch {
    return null;
  }
}

export function listSnapshots(root: string): SnapshotInfo[] {
  let entries: fs.Dirent[];
  try {
    entries = fs.readdirSync(snapshotsDir(root), { withFileTypes: true });
  } catch {
    return [];
  }
  return entries
    .filter((entry) => entry.isDirectory() && isSnapshotId(entry.name))
    .flatMap((entry) => {
      const manifest = readManifest(root, entry.name);
      const dbBytes = fileBytes(path.join(snapshotsDir(root), entry.name, "vocabulary.db"));
      return manifest && dbBytes !== null ? [infoOf(entry.name, manifest, dbBytes)] : [];
    })
    .sort((a, b) => (a.id < b.id ? 1 : -1));
}

export function restoreSnapshot(store: Store, id: string, options: SnapshotOptions = {}): void {
  const manifest = verifiedManifest(store.root, id);
  createSnapshot(store, "before-restore", options);
  for (const key of manifest.audioKeys) {
    const live = audioPath(store.root, key);
    if (!clipIsSound(live, key)) fs.rmSync(live, { force: true });
    copyClip(snapshotAudioPath(store.root, key), live, key);
  }
  const staged = path.join(store.root, "vocabulary.db.restoring");
  fs.copyFileSync(path.join(snapshotsDir(store.root), id, "vocabulary.db"), staged);
  store.close();
  fs.renameSync(staged, path.join(store.root, "vocabulary.db"));
}
