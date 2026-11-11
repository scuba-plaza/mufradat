import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { context } from "@/lib/server";
import { listSnapshots } from "@/lib/snapshots";
import { openCard, wordRows } from "@/lib/words";
import { DELETE, PUT } from "./route";

const remove = (id: string) => DELETE(new Request(`http://127.0.0.1/api/notes/${id}`, { method: "DELETE" }), { params: Promise.resolve({ id }) });

describe("deleting a card through the API", () => {
  let home: string;

  beforeEach(() => {
    home = fs.mkdtempSync(path.join(os.tmpdir(), "mufradat-notes-"));
    vi.stubEnv("XDG_CONFIG_HOME", path.join(home, "config"));
    vi.stubEnv("MUFRADAT_WORKSPACE", path.join(home, "workspace"));
  });
  afterEach(() => {
    vi.unstubAllEnvs();
    fs.rmSync(home, { recursive: true, force: true });
  });

  it("deletes the card and gives the word back as not started", async () => {
    const { store } = context();
    const id = openCard(store, { rank: 10 })!;
    const res = await remove(id);
    expect(res.status).toBe(204);
    expect(store.getNote(id)).toBeNull();
    expect(wordRows(store)[9]).toMatchObject({ rank: 10, id: null, status: "todo" });
  });

  it("answers the same when the card is already gone", async () => {
    const id = openCard(context().store, { rank: 10 })!;
    await remove(id);
    expect((await remove(id)).status).toBe(204);
    expect((await remove("no-such-card")).status).toBe(204);
  });

  it("takes no snapshot of its own when a card is reset", async () => {
    const { store } = context();
    const id = openCard(store, { rank: 10 })!;
    await remove(id);
    expect(store.getNote(id)).toBeNull();
    expect(listSnapshots(store.root).filter((s) => s.reason !== "daily")).toEqual([]);
  });

  it("saves the Arabic hint of the card", async () => {
    const { store } = context();
    const id = openCard(store, { rank: 10 })!;
    const edit = (body: Record<string, unknown>) =>
      PUT(new Request(`http://127.0.0.1/api/notes/${id}`, { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) }), { params: Promise.resolve({ id }) });
    const res = await edit({ hintAr: "ليس كَتَب" });
    expect(res.status).toBe(200);
    expect(store.getNote(id)!.hintAr).toBe("ليس كَتَب");
  });

  it("resets the card even when no backup could be made", async () => {
    const { store } = context();
    const id = openCard(store, { rank: 10 })!;
    fs.writeFileSync(path.join(store.root, "snapshots"), "in the way");
    const res = await remove(id);
    expect(res.status).toBe(204);
    expect(store.getNote(id)).toBeNull();
  });
});
