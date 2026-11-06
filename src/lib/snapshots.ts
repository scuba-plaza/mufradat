import Database from "better-sqlite3";
import { randomUUID } from "node:crypto";
import fs from "node:fs";
import path from "node:path";
import { audioFiles, audioKey, audioPath } from "./audio";
import { openBackupZip, refuse, type BackupEntry, type BackupZip } from "./backup-zip";
import { automaticSize, DEFAULT_LIMITS, selectRemovals, type Limits, type RetentionEntry } from "./retention";
import { openStore, SCHEMA_VERSION, type Store } from "./store";
import { SNAPSHOT_REASONS, type SnapshotInfo, type SnapshotReason, type SnapshotUsage } from "./types";

export interface SnapshotOptions {
  now?: Date;
  limits?: Limits;
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

function storeClip(bytes: Uint8Array, to: string): void {
  fs.mkdirSync(path.dirname(to), { recursive: true });
  const staged = `${to}.tmp`;
  fs.writeFileSync(staged, bytes);
  fs.renameSync(staged, to);
}

function copyClip(from: string, to: string, key: string): boolean {
  if (fs.existsSync(to)) return true;
  if (!fs.existsSync(from)) return false;
  const bytes = fs.readFileSync(from);
  if (audioKey(bytes) !== key) return false;
  storeClip(bytes, to);
  return true;
}

const entryName = (key: string) => `audio/${key.slice(0, 2)}/${key}.mp3`;

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

function writeSnapshot(store: Store, reason: SnapshotReason, now: Date, id: string): SnapshotInfo {
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

export function localDay(date: Date): string {
  const two = (n: number) => String(n).padStart(2, "0");
  return `${date.getFullYear()}-${two(date.getMonth() + 1)}-${two(date.getDate())}`;
}

export function dailyDue(snapshots: Pick<SnapshotInfo, "reason" | "createdAt">[], now: Date, cards: number): boolean {
  const today = localDay(now);
  return cards > 0 && !snapshots.some((s) => s.reason === "daily" && localDay(new Date(s.createdAt)) === today);
}

function pruneQuietly(root: string, limits?: Limits): void {
  try {
    applyRetention(root, limits);
  } catch (error) {
    console.error(error);
  }
}

export function createSnapshot(store: Store, reason: SnapshotReason, options: SnapshotOptions = {}): SnapshotInfo {
  const now = options.now ?? new Date();
  const info = writeSnapshot(store, reason, now, freeId(store.root, now, reason));
  pruneQuietly(store.root, options.limits);
  return info;
}

const NEWER = "This backup was written by a newer Mufradat.";
const DAMAGED = "The backup's database is damaged.";

function inspectDatabase(file: string): { cards: number; approved: number; version: number } | null {
  try {
    const db = new Database(file, { readonly: true, fileMustExist: true });
    try {
      if (db.pragma("integrity_check", { simple: true }) !== "ok") return null;
      const extras = db.prepare("SELECT COUNT(*) AS n FROM sqlite_master WHERE type IN ('trigger', 'view')").get() as { n: number };
      if (extras.n > 0) return null;
      const counts = db
        .prepare("SELECT COUNT(*) AS cards, COALESCE(SUM(status = 'approved'), 0) AS approved FROM notes")
        .get() as { cards: number; approved: number };
      return { ...counts, version: db.pragma("user_version", { simple: true }) as number };
    } finally {
      db.close();
    }
  } catch {
    return null;
  }
}

function clipIsSound(file: string, key: string): boolean {
  return fs.existsSync(file) && audioKey(fs.readFileSync(file)) === key;
}

function verifiedManifest(root: string, id: string): Manifest {
  const manifest = isSnapshotId(id) ? readManifest(root, id) : null;
  if (!manifest) throw new Error("There is no such backup.");
  if (manifest.schema > SCHEMA_VERSION) throw new Error(NEWER);
  if (!inspectDatabase(path.join(snapshotsDir(root), id, "vocabulary.db"))) throw new Error(DAMAGED);
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

function readAll(root: string): { info: SnapshotInfo; manifest: Manifest }[] {
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
      return manifest && dbBytes !== null ? [{ info: infoOf(entry.name, manifest, dbBytes), manifest }] : [];
    })
    .sort((a, b) => (a.info.id < b.info.id ? 1 : -1));
}

export function listSnapshots(root: string): SnapshotInfo[] {
  return readAll(root).map((entry) => entry.info);
}

function dropUnheldClips(root: string): void {
  const held = new Set(readAll(root).flatMap((entry) => entry.manifest.audioKeys));
  for (const file of audioFiles(snapshotsDir(root))) if (!held.has(file.key)) fs.rmSync(file.path, { force: true });
}

export function deleteSnapshot(root: string, id: string): void {
  if (!isSnapshotId(id) || !readManifest(root, id)) throw new Error("There is no such backup.");
  fs.rmSync(path.join(snapshotsDir(root), id), { recursive: true, force: true });
  dropUnheldClips(root);
}

function entriesOf(root: string): RetentionEntry[] {
  return readAll(root).map(({ info, manifest }) => ({
    id: info.id,
    reason: info.reason,
    dbBytes: info.dbBytes,
    clips: Object.fromEntries(manifest.audioKeys.map((key) => [key, fileBytes(snapshotAudioPath(root, key)) ?? 0])),
  }));
}

export function applyRetention(root: string, limits: Limits = DEFAULT_LIMITS): string[] {
  const removed = selectRemovals(entriesOf(root), limits);
  for (const id of removed) fs.rmSync(path.join(snapshotsDir(root), id), { recursive: true, force: true });
  dropUnheldClips(root);
  return removed;
}

export function snapshotUsage(root: string, limits: Limits = DEFAULT_LIMITS): SnapshotUsage {
  const entries = entriesOf(root);
  const pool = audioFiles(snapshotsDir(root)).reduce((sum, file) => sum + file.bytes, 0);
  return {
    totalBytes: entries.reduce((sum, entry) => sum + entry.dbBytes, 0) + pool,
    automaticBytes: automaticSize(entries),
    budgetBytes: limits.budgetBytes,
  };
}

export function recordDownload(root: string, now: Date = new Date()): void {
  fs.mkdirSync(snapshotsDir(root), { recursive: true });
  fs.writeFileSync(path.join(snapshotsDir(root), "last-download"), now.toISOString());
}

export function lastDownload(root: string): string | null {
  try {
    return fs.readFileSync(path.join(snapshotsDir(root), "last-download"), "utf8");
  } catch {
    return null;
  }
}

export function backupEntries(root: string, id: string): BackupEntry[] {
  const manifest = isSnapshotId(id) ? readManifest(root, id) : null;
  if (!manifest) throw new Error("There is no such backup.");
  if (!manifest.audioKeys.every((key) => fs.existsSync(snapshotAudioPath(root, key)))) {
    throw new Error("The backup is missing or has damaged audio clips.");
  }
  const dir = path.join(snapshotsDir(root), id);
  return [
    { name: "manifest.json", file: path.join(dir, "manifest.json") },
    { name: "vocabulary.db", file: path.join(dir, "vocabulary.db") },
    ...manifest.audioKeys.map((key) => ({ name: entryName(key), file: snapshotAudioPath(root, key) })),
  ];
}

export function incomingFile(root: string): string {
  fs.mkdirSync(snapshotsDir(root), { recursive: true });
  return path.join(snapshotsDir(root), `.incoming-${randomUUID()}.zip`);
}

function openable(dir: string): boolean {
  try {
    const store = openStore(dir);
    try {
      store.allNotes();
      store.check();
      return true;
    } finally {
      store.close();
    }
  } catch {
    return false;
  }
}

function incomingManifest(bytes: Buffer): { schema: number; audioKeys: string[]; unavailable: number } | null {
  let raw: unknown;
  try {
    raw = JSON.parse(bytes.toString("utf8"));
  } catch {
    return null;
  }
  if (typeof raw !== "object" || raw === null) return null;
  const { schema, audioKeys, unavailable } = raw as Record<string, unknown>;
  const keys = Array.isArray(audioKeys) ? audioKeys : null;
  const wellFormed = keys !== null && keys.every((key) => typeof key === "string") && new Set(keys).size === keys.length;
  if (typeof schema !== "number" || !Number.isInteger(schema) || schema < 1 || !wellFormed) return null;
  return { schema, audioKeys: keys as string[], unavailable: typeof unavailable === "number" ? unavailable : 0 };
}

function importFrom(root: string, backup: BackupZip, options: SnapshotOptions): SnapshotInfo {
  const incoming = incomingManifest(backup.read("manifest.json")) ?? refuse();
  if (incoming.schema > SCHEMA_VERSION) throw new Error(NEWER);
  const wanted = new Set(incoming.audioKeys.map(entryName));
  const held = backup.names.filter((name) => name.startsWith("audio/"));
  if (held.length !== wanted.size || !held.every((name) => wanted.has(name))) return refuse();

  const now = options.now ?? new Date();
  const id = freeId(root, now, "imported");
  const partial = path.join(snapshotsDir(root), `.partial-${id}`);
  fs.mkdirSync(partial, { recursive: true });
  try {
    const database = path.join(partial, "vocabulary.db");
    backup.copyTo("vocabulary.db", database);
    const inspected = inspectDatabase(database);
    if (!inspected) throw new Error(DAMAGED);
    if (inspected.version > SCHEMA_VERSION) throw new Error(NEWER);
    if (!openable(partial)) throw new Error(DAMAGED);
    for (const key of incoming.audioKeys) {
      const bytes = backup.read(entryName(key));
      if (audioKey(bytes) !== key) return refuse();
      storeClip(bytes, snapshotAudioPath(root, key));
    }
    const manifest: Manifest = {
      schema: incoming.schema,
      createdAt: now.toISOString(),
      reason: "imported",
      cards: inspected.cards,
      approved: inspected.approved,
      audioKeys: incoming.audioKeys,
      unavailable: incoming.unavailable,
    };
    fs.writeFileSync(path.join(partial, "manifest.json"), JSON.stringify(manifest));
    fs.renameSync(partial, path.join(snapshotsDir(root), id));
    return infoOf(id, manifest, fs.statSync(path.join(snapshotsDir(root), id, "vocabulary.db")).size);
  } catch (error) {
    fs.rmSync(partial, { recursive: true, force: true });
    dropUnheldClips(root);
    throw error;
  }
}

export function importBackup(root: string, file: string, options: SnapshotOptions = {}): SnapshotInfo {
  const backup = openBackupZip(file);
  try {
    return importFrom(root, backup, options);
  } finally {
    backup.close();
  }
}

export function restoreSnapshot(store: Store, id: string, options: SnapshotOptions = {}): void {
  const manifest = verifiedManifest(store.root, id);
  const now = options.now ?? new Date();
  writeSnapshot(store, "before-restore", now, freeId(store.root, now, "before-restore"));
  for (const key of manifest.audioKeys) {
    const live = audioPath(store.root, key);
    if (!clipIsSound(live, key)) fs.rmSync(live, { force: true });
    copyClip(snapshotAudioPath(store.root, key), live, key);
  }
  const staged = path.join(store.root, "vocabulary.db.restoring");
  fs.copyFileSync(path.join(snapshotsDir(store.root), id, "vocabulary.db"), staged);
  store.close();
  fs.renameSync(staged, path.join(store.root, "vocabulary.db"));
  pruneQuietly(store.root, options.limits);
}
