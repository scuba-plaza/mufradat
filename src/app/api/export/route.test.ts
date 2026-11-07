import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { audioKey, audioPath } from "@/lib/audio";
import Database from "better-sqlite3";
import { deckGuids, unzip } from "@/lib/testing/deck";
import { context } from "@/lib/server";
import type { Store } from "@/lib/store";
import { openCard } from "@/lib/words";
import { GET } from "./route";

const mp3 = Buffer.from([0xff, 0xfb, 0x90, 0x00, 1, 2, 3, 4]);

const request = (q?: string) => new Request(`http://127.0.0.1/api/export${q === undefined ? "" : `?q=${encodeURIComponent(q)}`}`);

function card(store: Store, rank: number, tags: string[], approved: boolean): string {
  const id = openCard(store, { rank })!;
  store.setAudio(id, "word", mp3, "tts");
  if (approved) expect(store.approve(id).ok).toBe(true);
  store.setTags(id, tags);
  return id;
}

describe("exporting through the API", () => {
  let home: string;

  beforeEach(() => {
    home = fs.mkdtempSync(path.join(os.tmpdir(), "mufradat-export-route-"));
    vi.stubEnv("XDG_CONFIG_HOME", path.join(home, "config"));
    vi.stubEnv("MUFRADAT_WORKSPACE", path.join(home, "workspace"));
  });
  afterEach(() => {
    vi.unstubAllEnvs();
    fs.rmSync(home, { recursive: true, force: true });
  });

  it("names the card and the slot when a clip's file is missing", async () => {
    const { store } = context();
    const id = openCard(store, { rank: 10 })!;
    store.setAudio(id, "word", mp3, "tts");
    expect(store.approve(id).ok).toBe(true);
    fs.rmSync(audioPath(store.root, audioKey(mp3)));
    const res = await GET(request());
    expect(res.status).toBe(400);
    expect(await res.json()).toEqual({
      error: `The audio for ${store.getNote(id)!.lemma} (word) is missing from the workspace. Generate or upload it again on the card.`,
    });
  });

  it("answers with the deck as a download and writes nothing to the workspace", async () => {
    const { store } = context();
    const id = openCard(store, { rank: 10 })!;
    store.setAudio(id, "word", mp3, "tts");
    expect(store.approve(id).ok).toBe(true);
    const res = await GET(request());
    expect(res.status).toBe(200);
    expect(res.headers.get("Content-Disposition")).toMatch(/^attachment; filename="arabic-deck-\d{4}-\d{2}-\d{2}\.apkg"$/);
    expect([...Buffer.from(await res.arrayBuffer()).subarray(0, 2)]).toEqual([0x50, 0x4b]);
    expect(fs.existsSync(path.join(store.root, "exports"))).toBe(false);
    expect(fs.readdirSync(store.root, { recursive: true }).filter((file) => String(file).endsWith(".apkg"))).toEqual([]);
  });

  it("refuses when nothing is approved", async () => {
    context();
    const res = await GET(request());
    expect(res.status).toBe(400);
    expect(await res.json()).toEqual({ error: "There are no approved cards to export." });
  });

  it("exports only the approved cards the search matches", async () => {
    const { store } = context();
    const x = card(store, 1, ["lesson_1"], true);
    const y = card(store, 2, ["lesson_1"], true);
    card(store, 3, ["lesson_2"], true);
    card(store, 4, ["lesson_1"], false);
    const res = await GET(request("tag:lesson_1"));
    expect(res.status).toBe(200);
    expect(deckGuids(Buffer.from(await res.arrayBuffer()))).toEqual([x, y].sort());
  });

  it("exports every match, not only the first page", async () => {
    const { store } = context();
    const ids = Array.from({ length: 31 }, (_, i) => card(store, i + 1, ["lesson_1"], true));
    card(store, 40, ["lesson_1"], false);
    const res = await GET(request("tag:lesson_1"));
    expect(deckGuids(Buffer.from(await res.arrayBuffer()))).toEqual(ids.sort());
  });

  it("exports every approved card for a blank search", async () => {
    const { store } = context();
    const ids = [card(store, 1, [], true), card(store, 2, ["lesson_1"], true)].sort();
    card(store, 3, [], false);
    for (const q of [undefined, "", "  "]) {
      const res = await GET(request(q));
      expect(deckGuids(Buffer.from(await res.arrayBuffer()))).toEqual(ids);
    }
  });

  it("says why a search that cannot be read exports nothing", async () => {
    card(context().store, 1, [], true);
    const res = await GET(request("pos:animal"));
    expect(res.status).toBe(400);
    expect(await res.json()).toEqual({
      error: 'pos: expects noun, verb, adj, adv, prep, conj, particle, pron, num, intj, phrase or det, not "animal".',
    });
  });

  it("refuses a search no approved card matches, also when a draft matches", async () => {
    const { store } = context();
    card(store, 1, ["lesson_1"], true);
    card(store, 2, ["lesson_2"], false);
    for (const q of ["tag:none_such", "tag:lesson_2"]) {
      const res = await GET(request(q));
      expect(res.status).toBe(400);
      expect(await res.json()).toEqual({ error: "No approved card matches this search." });
    }
  });

  it("numbers the cards without a rank by the whole workspace, so a filtered export keeps their due", async () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    const { store } = context();
    const make = (lemma: string, at: string, approved: boolean) => {
      vi.setSystemTime(new Date(at));
      const n = store.createNote({ lemma, english: "x", pos: "noun", rank: 0 });
      store.setAudio(n.id, "word", mp3, "tts");
      if (approved) expect(store.approve(n.id).ok).toBe(true);
      store.setTags(n.id, approved ? ["wanted"] : []);
      return n.id;
    };
    make("جَ", "2026-11-07T12:00:00Z", false);
    const wanted = make("دَ", "2026-11-07T13:00:00Z", true);
    vi.useRealTimers();
    const res = await GET(request("tag:wanted"));
    const db = new Database(unzip(Buffer.from(await res.arrayBuffer())).get("collection.anki2")!);
    const cards = db.prepare("SELECT c.due, n.guid FROM cards c JOIN notes n ON n.id = c.nid").all();
    db.close();
    expect(cards).toEqual([{ due: 5002, guid: wanted }]);
  });
});
