import { fail } from "@/lib/server";
import { listSnapshots } from "@/lib/snapshots";
import type { Store } from "@/lib/store";
import type { Note } from "@/lib/types";

export type IdContext = { params: Promise<{ id: string }> };

export const noteOf = (store: Store, id: unknown): Note | null => (typeof id === "string" ? store.getNote(id) : null);

export const noSuchCard = () => fail("There is no such card.", 404);

export const hasBackup = (root: string, id: string) => listSnapshots(root).some((s) => s.id === id);

export const noSuchBackup = () => fail("There is no such backup.", 404);

export const refusal = (e: unknown, fallback: string) => fail(e instanceof Error ? e.message : fallback);
