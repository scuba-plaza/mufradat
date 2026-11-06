import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { afterEach, describe, expect, it } from "vitest";
import { audioPath } from "./audio";
import { createSnapshot, listSnapshots, restoreSnapshot, snapshotAudioPath } from "./snapshots";
import { openStore } from "./store";

const mp3 = Buffer.from([0xff, 0xfb, 0x90, 0x00, 1, 2, 3, 4]);
const kitab = {
  lemma: "كِتَاب", english: "book", pos: "noun" as const, gender: "m", rank: 120,
  forms: [{ label: "pl.", arabic: "كُتُب" }],
};

const roots: string[] = [];

afterEach(() => {
  for (const root of roots.splice(0)) fs.rmSync(root, { recursive: true, force: true });
});

function workspace() {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "mufradat-snapshots-"));
  roots.push(root);
  return { root, store: openStore(root) };
}

const at = (iso: string) => ({ now: new Date(iso) });

describe("restoring a snapshot", () => {
  it("brings a reset card back with its forms", () => {
    const { root, store } = workspace();
    const card = store.createNote(kitab);
    const snapshot = createSnapshot(store, "manual", at("2026-11-06T10:00:00Z"));
    store.deleteNote(card.id);
    restoreSnapshot(store, snapshot.id);
    const back = openStore(root);
    expect(back.getNote(card.id)?.forms.map((f) => [f.label, f.arabic])).toEqual([["pl.", "كُتُب"]]);
  });

  it("plays the card's clip after a restore, also after Delete unused audio", () => {
    const { root, store } = workspace();
    const card = store.createNote(kitab);
    store.setAudio(card.id, "word", mp3, "tts");
    const snapshot = createSnapshot(store, "manual", at("2026-11-06T10:00:00Z"));
    store.deleteNote(card.id);
    store.deleteUnusedAudio();
    restoreSnapshot(store, snapshot.id);
    const back = openStore(root);
    const key = back.getNote(card.id)!.audio.word.key!;
    expect(fs.readFileSync(audioPath(root, key))).toEqual(mp3);
  });

  it("brings every clip back when the whole audio folder is gone", () => {
    const { root, store } = workspace();
    const card = store.createNote(kitab);
    store.setAudio(card.id, "word", mp3, "tts");
    const snapshot = createSnapshot(store, "manual", at("2026-11-06T10:00:00Z"));
    fs.rmSync(path.join(root, "audio"), { recursive: true });
    restoreSnapshot(store, snapshot.id);
    const back = openStore(root);
    const key = back.getNote(card.id)!.audio.word.key!;
    expect(fs.readFileSync(audioPath(root, key))).toEqual(mp3);
  });

  it("snapshots and restores a workspace that has cards and no audio folder at all", () => {
    const { root, store } = workspace();
    const card = store.createNote(kitab);
    const snapshot = createSnapshot(store, "manual", at("2026-11-06T10:00:00Z"));
    expect(snapshot.clips).toBe(0);
    store.deleteNote(card.id);
    restoreSnapshot(store, snapshot.id);
    expect(openStore(root).getNote(card.id)?.lemma).toBe("كِتَاب");
  });
});

describe("undoing a restore", () => {
  it("saves the current state first, and restoring that undoes the restore", () => {
    const { root, store } = workspace();
    const first = store.createNote(kitab);
    const snapshot = createSnapshot(store, "manual", at("2026-11-06T10:00:00Z"));
    const second = store.createNote({ ...kitab, lemma: "قَلَم", english: "pen", forms: [] });
    restoreSnapshot(store, snapshot.id, at("2026-11-06T11:00:00Z"));
    const afterRestore = openStore(root);
    expect(afterRestore.getNote(second.id)).toBeNull();
    const saved = listSnapshots(root).find((s) => s.reason === "before-restore")!;
    restoreSnapshot(afterRestore, saved.id, at("2026-11-06T12:00:00Z"));
    const undone = openStore(root);
    expect([first.id, second.id].map((id) => undone.getNote(id)?.lemma)).toEqual(["كِتَاب", "قَلَم"]);
  });
});

describe("refusing a restore", () => {
  const clipsGone = "The backup is missing or has damaged audio clips.";
  const tamperings: [string, (root: string, id: string, key: string) => void, string][] = [
    [
      "a snapshot from a newer version",
      (root, id) => {
        const file = path.join(root, "snapshots", id, "manifest.json");
        fs.writeFileSync(file, JSON.stringify({ ...JSON.parse(fs.readFileSync(file, "utf8")), schema: 99 }));
      },
      "This backup was written by a newer Mufradat.",
    ],
    [
      "a damaged database",
      (root, id) => fs.writeFileSync(path.join(root, "snapshots", id, "vocabulary.db"), "not a database"),
      "The backup's database is damaged.",
    ],
    ["a damaged clip", (root, _id, key) => fs.writeFileSync(snapshotAudioPath(root, key), Buffer.from("x")), clipsGone],
    ["a missing clip", (root, _id, key) => fs.rmSync(snapshotAudioPath(root, key)), clipsGone],
  ];

  it.each(tamperings)("refuses %s and changes nothing", (_name, tamper, message) => {
    const { root, store } = workspace();
    const card = store.createNote(kitab);
    store.setAudio(card.id, "word", mp3, "tts");
    const snapshot = createSnapshot(store, "manual", at("2026-11-06T10:00:00Z"));
    const later = store.createNote({ ...kitab, lemma: "قَلَم", english: "pen", forms: [] });
    tamper(root, snapshot.id, store.getNote(card.id)!.audio.word.key!);
    expect(() => restoreSnapshot(store, snapshot.id)).toThrow(message);
    expect(store.getNote(later.id)?.lemma).toBe("قَلَم");
    expect(listSnapshots(root).map((s) => s.id)).toEqual([snapshot.id]);
  });

  it.each(["../x", "20260101T000000Z-manual"])("refuses %s as a snapshot that does not exist", (id) => {
    const { store } = workspace();
    store.createNote(kitab);
    createSnapshot(store, "manual", at("2026-11-06T10:00:00Z"));
    expect(() => restoreSnapshot(store, id)).toThrow("There is no such backup.");
  });
});
