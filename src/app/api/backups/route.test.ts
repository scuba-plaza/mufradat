import fs from "node:fs";
import path from "node:path";
import { afterEach, describe, expect, it, vi } from "vitest";
import { audioKey, audioPath } from "@/lib/audio";
import { openBackupZip } from "@/lib/backup-zip";
import { context } from "@/lib/server";
import { snapshotAudioPath } from "@/lib/snapshots";
import type { SnapshotInfo } from "@/lib/types";
import { openCard } from "@/lib/words";
import { mp3, tempWorkspace, withParams } from "@/lib/testing/workspace";
import { DELETE, GET as download } from "./[id]/route";
import { GET, POST } from "./route";

const removeBackup = (id: string) =>
  DELETE(new Request(`http://127.0.0.1/api/backups/${id}`, { method: "DELETE" }), withParams({ id }));

const downloadBackup = (id: string) =>
  download(new Request(`http://127.0.0.1/api/backups/${id}`), withParams({ id }));

const lastDownloadedAt = async () => (await (await GET()).json()).lastDownloadedAt;

const manuals = async () =>
  ((await (await GET()).json()).snapshots as SnapshotInfo[]).filter((s) => s.reason === "manual");

describe("the backups API", () => {
  const home = tempWorkspace();
  afterEach(() => {
    vi.useRealTimers();
  });

  it("lists nothing in an empty workspace and says it is healthy", async () => {
    const res = await GET();
    expect(res.status).toBe(200);
    expect(await res.json()).toEqual({
      snapshots: [],
      usage: { automaticBytes: 0, totalBytes: 0, budgetBytes: 1073741824 },
      check: { cards: 0, approved: 0, integrity: "ok", missingAudio: [] },
      lastDownloadedAt: null,
    });
  });

  it("takes a manual backup on request and lists it with its counts", async () => {
    openCard(context().store, { rank: 10 });
    const res = await POST();
    expect(res.status).toBe(200);
    const { snapshot } = await res.json();
    expect(snapshot).toMatchObject({ reason: "manual", cards: 1, approved: 0 });
    expect((await manuals()).map((s) => s.id)).toEqual([snapshot.id]);
  });

  it("deletes a backup, and answers 404 for ids that are not backups", async () => {
    openCard(context().store, { rank: 10 });
    const { snapshot } = await (await POST()).json();
    for (const id of ["20260101T000000Z-manual", ".."]) {
      const res = await removeBackup(id);
      expect(res.status).toBe(404);
      expect(await res.json()).toEqual({ error: "There is no such backup." });
    }
    expect(await manuals()).toHaveLength(1);
    expect((await removeBackup(snapshot.id)).status).toBe(204);
    expect(await manuals()).toEqual([]);
  });

  it("shows a clip whose file is gone in the check", async () => {
    const { store } = context();
    const id = openCard(store, { rank: 10 })!;
    store.setAudio(id, "word", mp3, "tts");
    fs.rmSync(audioPath(store.root, audioKey(mp3)));
    const { check } = await (await GET()).json();
    expect(check.missingAudio).toEqual([
      { noteId: id, lemma: store.getNote(id)!.lemma, slot: "word", slotName: "word", key: audioKey(mp3) },
    ]);
  });

  it("takes one daily snapshot on the first request of a day that finds cards", async () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date(2026, 10, 6, 9, 0));
    const dailies = async () =>
      ((await (await GET()).json()).snapshots as SnapshotInfo[]).filter((s) => s.reason === "daily").length;
    expect(await dailies()).toBe(0);
    openCard(context().store, { rank: 10 });
    expect(await dailies()).toBe(1);
    expect(await dailies()).toBe(1);
    vi.setSystemTime(new Date(2026, 10, 7, 0, 5));
    expect(await dailies()).toBe(2);
  });

  it("does not fail a request when the daily snapshot cannot be written", () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date(2026, 10, 6, 9, 0));
    const { store } = context();
    openCard(store, { rank: 10 });
    fs.writeFileSync(path.join(store.root, "snapshots"), "in the way");
    const logged = vi.spyOn(console, "error").mockImplementation(() => {});
    expect(() => context()).not.toThrow();
    context();
    expect(logged).toHaveBeenCalledTimes(1);
    logged.mockRestore();
  });

  it("downloads a backup as one zip with the manifest, the database and the clips", async () => {
    const { store } = context();
    const id = openCard(store, { rank: 10 })!;
    store.setAudio(id, "word", mp3, "tts");
    const { snapshot } = await (await POST()).json();
    const res = await downloadBackup(snapshot.id);
    expect(res.status).toBe(200);
    expect(res.headers.get("content-type")).toBe("application/zip");
    expect(res.headers.get("content-disposition")).toBe(`attachment; filename="mufradat-backup-${snapshot.id}.zip"`);
    const file = path.join(home(), "download.zip");
    fs.writeFileSync(file, Buffer.from(await res.arrayBuffer()));
    const backup = openBackupZip(file);
    const clip = `audio/${audioKey(mp3).slice(0, 2)}/${audioKey(mp3)}.mp3`;
    expect([...backup.names].sort()).toEqual([clip, "manifest.json", "vocabulary.db"]);
    expect(JSON.parse(backup.read("manifest.json").toString()).cards).toBe(1);
    expect(backup.read(clip)).toEqual(mp3);
    backup.close();
  });

  it("remembers when a backup was last downloaded, and answers 404 for an unknown one", async () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date("2026-11-06T10:00:00Z"));
    openCard(context().store, { rank: 10 });
    const { snapshot } = await (await POST()).json();
    expect(await lastDownloadedAt()).toBeNull();
    expect((await downloadBackup("20260101T000000Z-manual")).status).toBe(404);
    expect(await lastDownloadedAt()).toBeNull();
    await (await downloadBackup(snapshot.id)).arrayBuffer();
    expect(await lastDownloadedAt()).toBe("2026-11-06T10:00:00.000Z");
  });

  it.each([
    ["missing", (clip: string) => fs.rmSync(clip)],
    ["damaged", (clip: string) => fs.writeFileSync(clip, Buffer.from("x"))],
  ])("answers 400 instead of a download the other end would refuse when a clip in the backup is %s", async (_what, spoil) => {
    const { store } = context();
    const id = openCard(store, { rank: 10 })!;
    store.setAudio(id, "word", mp3, "tts");
    const { snapshot } = await (await POST()).json();
    spoil(snapshotAudioPath(store.root, audioKey(mp3)));
    const res = await downloadBackup(snapshot.id);
    expect(res.status).toBe(400);
    expect(await res.json()).toEqual({ error: "The backup is missing or has damaged audio clips." });
    expect(await lastDownloadedAt()).toBeNull();
  });

  it("still downloads when the note of the download cannot be written", async () => {
    const { store } = context();
    openCard(store, { rank: 10 });
    const { snapshot } = await (await POST()).json();
    fs.mkdirSync(path.join(store.root, "snapshots", "last-download"));
    const logged = vi.spyOn(console, "error").mockImplementation(() => {});
    const res = await downloadBackup(snapshot.id);
    expect(res.status).toBe(200);
    const file = path.join(home(), "download.zip");
    fs.writeFileSync(file, Buffer.from(await res.arrayBuffer()));
    expect(openBackupZip(file).names).toContain("manifest.json");
    expect(logged).toHaveBeenCalledTimes(1);
    logged.mockRestore();
  });

  it("does not say a backup was downloaded when the download is cancelled part way", async () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date("2026-11-06T10:00:00Z"));
    openCard(context().store, { rank: 10 });
    const { snapshot } = await (await POST()).json();
    const reader = (await downloadBackup(snapshot.id)).body!.getReader();
    await reader.read();
    await reader.cancel();
    expect(await lastDownloadedAt()).toBeNull();
  });
});
