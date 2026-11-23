import fs from "node:fs";
import path from "node:path";
import { describe, expect, it, vi } from "vitest";
import { audioKey, audioPath } from "@/lib/audio";
import { context } from "@/lib/server";
import { listSnapshots } from "@/lib/snapshots";
import { openCard } from "@/lib/words";
import { mp3, tempWorkspace, withParams } from "@/lib/testing/workspace";
import { GET as download } from "../[id]/route";
import { POST as restoreRoute } from "../[id]/restore/route";
import { POST as createBackup } from "../route";
import { POST } from "./route";

const upload = (body: BodyInit, headers: Record<string, string> = {}) =>
  POST(new Request("http://127.0.0.1/api/backups/import", { method: "POST", body, headers }));

const downloaded = async (id: string) =>
  Buffer.from(await (await download(new Request(`http://127.0.0.1/api/backups/${id}`), withParams({ id }))).arrayBuffer());

const restore = (id: string) =>
  restoreRoute(new Request(`http://127.0.0.1/api/backups/${id}/restore`, { method: "POST" }), withParams({ id }));

describe("importing a backup through the API", () => {
  const home = tempWorkspace();

  async function backupOfA() {
    const { store } = context();
    const id = openCard(store, { rank: 10 })!;
    store.setAudio(id, "word", mp3, "tts");
    expect(store.approve(id).ok).toBe(true);
    const { snapshot } = await (await createBackup()).json();
    return { id, zip: await downloaded(snapshot.id) };
  }

  const moveToWorkspaceB = () => vi.stubEnv("MUFRADAT_WORKSPACE", path.join(home(), "workspace-b"));

  it("imports a downloaded backup into a fresh workspace, and a restore brings everything back", async () => {
    const { id, zip } = await backupOfA();
    moveToWorkspaceB();
    const res = await upload(zip);
    expect(res.status).toBe(200);
    const { snapshot } = await res.json();
    expect(snapshot).toMatchObject({ reason: "imported", cards: 1, approved: 1, clips: 1 });
    expect((await restore(snapshot.id)).status).toBe(200);
    const fresh = context().store;
    expect(fresh.getNote(id)).toMatchObject({ status: "approved" });
    expect(fs.readFileSync(audioPath(fresh.root, audioKey(mp3)))).toEqual(mp3);
  });

  it("answers 400 for a body that is not a backup, or is cut short, and leaves nothing behind", async () => {
    const { zip } = await backupOfA();
    moveToWorkspaceB();
    for (const body of [Buffer.from("not a zip"), zip.subarray(0, Math.floor(zip.length / 2))]) {
      const res = await upload(body);
      expect(res.status).toBe(400);
      expect(await res.json()).toEqual({ error: "This is not a Mufradat backup." });
    }
    const { store } = context();
    expect(listSnapshots(store.root)).toEqual([]);
    expect(fs.readdirSync(path.join(store.root, "snapshots"))).toEqual([]);
  });

  it("refuses a request from another origin, or one whose origin cannot be read, and writes nothing", async () => {
    const { zip } = await backupOfA();
    moveToWorkspaceB();
    for (const origin of ["http://evil.example", "null"]) {
      const res = await upload(zip, { origin, host: "127.0.0.1" });
      expect(res.status).toBe(403);
      expect(await res.json()).toEqual({ error: "Cross-site request refused." });
    }
    expect(listSnapshots(context().store.root)).toEqual([]);
    expect((await upload(zip, { origin: "http://127.0.0.1", host: "127.0.0.1" })).status).toBe(200);
  });

  it("answers 413 for a file that announces more than 3 GiB, and writes nothing", async () => {
    const res = await upload("x", { "content-length": String(3 * 1024 ** 3 + 1) });
    expect(res.status).toBe(413);
    expect(await res.json()).toEqual({ error: "That file is larger than 3 GiB." });
    expect(fs.existsSync(path.join(context().store.root, "snapshots"))).toBe(false);
  });
});
