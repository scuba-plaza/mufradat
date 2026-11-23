import type { SnapshotReason } from "./types";

export interface RetentionEntry {
  id: string;
  reason: SnapshotReason;
  dbBytes: number;
  clips: Record<string, number>;
}

export interface Limits {
  dailyKeep: number;
  beforeKeep: number;
  budgetBytes: number;
}

export const DEFAULT_LIMITS: Limits = { dailyKeep: 14, beforeKeep: 14, budgetBytes: 1024 ** 3 };

function byId(a: RetentionEntry, b: RetentionEntry): number {
  if (a.id === b.id) return 0;
  return a.id < b.id ? -1 : 1;
}

const AUTOMATIC: readonly SnapshotReason[] = ["daily", "before-reset", "before-restore"];
const isAutomatic = (e: RetentionEntry) => AUTOMATIC.includes(e.reason);

const groupOf = (e: RetentionEntry) => (e.reason === "daily" ? "daily" : "before");

export function automaticSize(entries: RetentionEntry[]): number {
  return sizeOf(entries.filter(isAutomatic));
}

function sizeOf(entries: RetentionEntry[]): number {
  const clips = new Map<string, number>();
  for (const e of entries) for (const [key, bytes] of Object.entries(e.clips)) clips.set(key, bytes);
  return entries.reduce((sum, e) => sum + e.dbBytes, 0) + [...clips.values()].reduce((sum, bytes) => sum + bytes, 0);
}

export function selectRemovals(entries: RetentionEntry[], limits: Limits): string[] {
  const automatic = entries.filter(isAutomatic).sort(byId);
  const keeps = { daily: limits.dailyKeep, before: limits.beforeKeep };
  const removed = new Set<string>();
  const newest = new Set<string>();
  for (const group of ["daily", "before"] as const) {
    const members = automatic.filter((e) => groupOf(e) === group);
    for (const old of members.slice(0, Math.max(0, members.length - keeps[group]))) removed.add(old.id);
    if (members.length > 0) newest.add(members[members.length - 1].id);
  }
  while (sizeOf(automatic.filter((e) => !removed.has(e.id))) > limits.budgetBytes) {
    const next = automatic.find((e) => !removed.has(e.id) && !newest.has(e.id));
    if (!next) break;
    removed.add(next.id);
  }
  return automatic.filter((e) => removed.has(e.id)).map((e) => e.id);
}
