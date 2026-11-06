import { NextResponse } from "next/server";
import { loadConfig, type Config } from "./config";
import { createSnapshot, dailyDue, listSnapshots, localDay } from "./snapshots";
import { openStore, type Store } from "./store";

interface Entry {
  workspace: string;
  store: Store;
  dailyDay?: string;
}

const g = globalThis as unknown as { __mufradat?: Entry };

function ensureDaily(entry: Entry, now: Date): void {
  const day = localDay(now);
  if (entry.dailyDay === day) return;
  const { cards } = entry.store.counts();
  if (cards === 0) return;
  entry.dailyDay = day;
  try {
    if (dailyDue(listSnapshots(entry.store.root), now, cards)) createSnapshot(entry.store, "daily", { now });
  } catch (error) {
    console.error(error);
  }
}

/** The config as it is now, and the store for its workspace. */
export function context(): { cfg: Config; store: Store } {
  const cfg = loadConfig();
  if (!g.__mufradat || g.__mufradat.workspace !== cfg.workspace || !g.__mufradat.store.isOpen()) {
    g.__mufradat?.store.close();
    g.__mufradat = { workspace: cfg.workspace, store: openStore(cfg.workspace) };
  }
  ensureDaily(g.__mufradat, new Date());
  return { cfg, store: g.__mufradat.store };
}

export const fail = (message: string, status = 400) => NextResponse.json({ error: message }, { status });

export async function readJson(req: Request): Promise<Record<string, unknown>> {
  try {
    const v = await req.json();
    return v && typeof v === "object" ? (v as Record<string, unknown>) : {};
  } catch {
    return {};
  }
}

/** Runs a handler and turns any thrown error into a JSON error response. */
export async function handle(fn: () => Promise<Response> | Response): Promise<Response> {
  try {
    return await fn();
  } catch (e) {
    return fail(e instanceof Error ? e.message : "Something went wrong.", 500);
  }
}
