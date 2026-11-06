import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { context } from "@/lib/server";
import { listSnapshots, restoreSnapshot } from "@/lib/snapshots";
import { openCard, wordRows } from "@/lib/words";
import { DELETE } from "./route";

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

  it("takes a before-reset snapshot first, and restoring it brings the card back", async () => {
    const { store } = context();
    const id = openCard(store, { rank: 10 })!;
    await remove(id);
    const saved = listSnapshots(store.root).filter((s) => s.reason === "before-reset");
    expect(saved).toHaveLength(1);
    expect(store.getNote(id)).toBeNull();
    restoreSnapshot(store, saved[0].id);
    expect(context().store.getNote(id)).not.toBeNull();
  });

  it("takes no snapshot for a card that is already gone", async () => {
    const { store } = context();
    const id = openCard(store, { rank: 10 })!;
    await remove(id);
    await remove(id);
    await remove("no-such-card");
    expect(listSnapshots(store.root).filter((s) => s.reason === "before-reset")).toHaveLength(1);
  });

  it("refuses to reset when the backup cannot be made, and changes nothing", async () => {
    const { store } = context();
    const id = openCard(store, { rank: 10 })!;
    fs.writeFileSync(path.join(store.root, "snapshots"), "in the way");
    const logged = vi.spyOn(console, "error").mockImplementation(() => {});
    const res = await remove(id);
    logged.mockRestore();
    expect(res.status).toBe(500);
    expect((await res.json()).error).toContain("The card was not reset because a backup could not be made first");
    expect(store.getNote(id)).not.toBeNull();
  });
});
