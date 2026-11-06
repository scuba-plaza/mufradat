import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { context } from "@/lib/server";
import { openCard } from "@/lib/words";
import { POST as createBackup } from "../../route";
import { POST } from "./route";

const restore = (id: string) =>
  POST(new Request(`http://127.0.0.1/api/backups/${id}/restore`, { method: "POST" }), { params: Promise.resolve({ id }) });

describe("restoring a backup through the API", () => {
  let home: string;

  beforeEach(() => {
    home = fs.mkdtempSync(path.join(os.tmpdir(), "mufradat-restore-route-"));
    vi.stubEnv("XDG_CONFIG_HOME", path.join(home, "config"));
    vi.stubEnv("MUFRADAT_WORKSPACE", path.join(home, "workspace"));
  });
  afterEach(() => {
    vi.unstubAllEnvs();
    fs.rmSync(home, { recursive: true, force: true });
  });

  it("restores and leaves the app showing the restored cards with a fresh store", async () => {
    const { store } = context();
    const id = openCard(store, { rank: 10 })!;
    const { snapshot } = await (await createBackup()).json();
    store.deleteNote(id);
    const res = await restore(snapshot.id);
    expect(res.status).toBe(200);
    expect(context().store).not.toBe(store);
    expect(context().store.getNote(id)).not.toBeNull();
  });

  it("answers 404 for a backup that does not exist", async () => {
    const res = await restore("20260101T000000Z-manual");
    expect(res.status).toBe(404);
    expect(await res.json()).toEqual({ error: "There is no such backup." });
  });

  it("refuses a backup from a newer version, saying why, and changes nothing", async () => {
    const { store } = context();
    openCard(store, { rank: 10 });
    const { snapshot } = await (await createBackup()).json();
    const later = openCard(store, { rank: 11 })!;
    const manifest = path.join(store.root, "snapshots", snapshot.id, "manifest.json");
    fs.writeFileSync(manifest, JSON.stringify({ ...JSON.parse(fs.readFileSync(manifest, "utf8")), schema: 99 }));
    const res = await restore(snapshot.id);
    expect(res.status).toBe(400);
    expect(await res.json()).toEqual({ error: "This backup was written by a newer Mufradat." });
    expect(context().store.getNote(later)).not.toBeNull();
  });
});
