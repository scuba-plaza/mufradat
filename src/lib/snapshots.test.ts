import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { afterEach, describe, expect, it } from "vitest";
import { audioPath } from "./audio";
import { createSnapshot, restoreSnapshot } from "./snapshots";
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
