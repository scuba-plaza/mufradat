import Database from "better-sqlite3";
import fs from "node:fs";
import { describe, expect, it } from "vitest";
import { buildApkg } from "./anki";
import { audioKey } from "./audio";
import { openStore } from "./store";
import { unzip } from "./testing/deck";
import type { Note } from "./types";

const FIELDS = [
  "Arabic", "English", "Hint", "Forms", "Example", "ExampleEnglish", "WordAudio", "FormsAudio",
  "ExampleAudio", "Pos", "Details", "Production", "Check", "Rank", "NoteID",
];
const clip = (n: number) => Buffer.from([0xff, 0xfb, 0x90, 0x00, n, n, n, n]);

function collection(apkg: Buffer) {
  const files = unzip(apkg);
  const db = new Database(files.get("collection.anki2")!);
  const notes = db.prepare("SELECT * FROM notes ORDER BY id").all() as { flds: string; tags: string; guid: string }[];
  const cards = db.prepare("SELECT c.ord, c.due, n.guid FROM cards c JOIN notes n ON n.id = c.nid ORDER BY n.guid, c.ord").all() as
    { ord: number; due: number; guid: string }[];
  const col = db.prepare("SELECT models, decks FROM col").get() as { models: string; decks: string };
  const model = Object.values(JSON.parse(col.models))[0] as any;
  const decks = Object.values(JSON.parse(col.decks)) as any[];
  const media = JSON.parse(files.get("media")!.toString()) as Record<string, string>;
  return { files, notes, cards, model, decks, media, fieldsOf: (i: number) => Object.fromEntries(notes[i].flds.split("\x1f").map((v, j) => [FIELDS[j], v])) };
}

const options = { deckName: "Arabic MSA Core", deckId: 1_700_000_000_123, modelId: 1_700_000_000_456 };

function approvedStore() {
  const s = openStore(":memory:");
  const n = s.createNote({
    lemma: "كِتَاب", english: "book & more", pos: "noun", gender: "m", root: "ك ت ب", rank: 120,
    forms: [{ label: "pl.", arabic: "كُتُب" }, { label: "dual", arabic: "كِتَابَانِ" }],
  });
  s.updateNote(n.id, { hint: "not كتابة", example: "هٰذَا [كِتَابٌ] جَمِيلٌ", exampleEn: "this is a <nice> book" });
  s.setAudio(n.id, "word", clip(1), "tts");
  s.setAudio(n.id, n.forms[0].id, clip(2), "tts");
  s.setAudio(n.id, n.forms[1].id, clip(3), "user");
  s.setAudio(n.id, "example", clip(4), "tts");
  s.approve(n.id);
  return { s, id: n.id };
}
const read = (s: ReturnType<typeof openStore>) => (key: string) => fs.readFileSync(`${s.root}/audio/${key.slice(0, 2)}/${key}.mp3`);

describe("exporting approved cards", () => {
  it("fills every field of the Arabic MSA Word note type", () => {
    const { s, id } = approvedStore();
    const apkg = buildApkg([s.getNote(id)!], read(s), options);
    const c = collection(apkg);
    expect(c.model.name).toBe("Arabic MSA Word");
    expect(c.model.flds.map((f: any) => f.name)).toEqual(FIELDS);
    const f = c.fieldsOf(0);
    const k = (n: number) => audioKey(clip(n));
    expect(f).toEqual({
      Arabic: "كِتَاب",
      English: "book &amp; more",
      Hint: 'not <span class="ar" lang="ar" dir="rtl">كتابة</span>',
      Forms:
        '<span class="form"><span class="lbl">pl.</span><span class="ar" lang="ar" dir="rtl">كُتُب</span></span> ' +
        '<span class="form"><span class="lbl">dual</span><span class="ar" lang="ar" dir="rtl">كِتَابَانِ</span></span>',
      Example: "هٰذَا <b>كِتَابٌ</b> جَمِيلٌ",
      ExampleEnglish: "this is a &lt;nice&gt; book",
      WordAudio: `[sound:${k(1)}.mp3]`,
      FormsAudio: `[sound:${k(2)}.mp3][sound:${k(3)}.mp3]`,
      ExampleAudio: `[sound:${k(4)}.mp3]`,
      Pos: "noun",
      Details: 'masc. · root <span class="ar" lang="ar" dir="rtl">ك ت ب</span>',
      Production: "y",
      Check: "",
      Rank: "120",
      NoteID: id,
    });
  });

  it("writes a card's tags after its pos and rank tags", () => {
    const { s, id } = approvedStore();
    s.setTags(id, ["lesson_1", "grammar"]);
    expect(collection(buildApkg([s.getNote(id)!], read(s), options)).notes[0].tags).toBe(" pos::noun rank::0001-0500 grammar lesson_1 ");
  });

  it("does not repeat a tag the deck already has", () => {
    const { s, id } = approvedStore();
    s.setTags(id, ["pos::noun"]);
    expect(collection(buildApkg([s.getNote(id)!], read(s), options)).notes[0].tags).toBe(" pos::noun rank::0001-0500 ");
  });

  it("gives the same bytes whatever order the tags were set in", () => {
    const { s, id } = approvedStore();
    s.setTags(id, ["lesson_1", "grammar"]);
    const first = buildApkg([s.getNote(id)!], read(s), options);
    s.setTags(id, ["grammar", "lesson_1"]);
    expect(buildApkg([s.getNote(id)!], read(s), options).equals(first)).toBe(true);
  });

  it("keeps the card templates and styling of the existing note type", () => {
    const { s, id } = approvedStore();
    const c = collection(buildApkg([s.getNote(id)!], read(s), options));
    const dir = "data/template/";
    const file = (n: string) => fs.readFileSync(dir + n, "utf8");
    expect(c.model.tmpls.map((t: any) => t.name)).toEqual(["Recognition", "Production"]);
    expect(c.model.tmpls[0].qfmt).toBe(file("recognition_front.html"));
    expect(c.model.tmpls[0].afmt).toBe(file("recognition_back.html") + file("panels.html"));
    expect(c.model.tmpls[1].qfmt).toBe(file("production_front.html"));
    expect(c.model.tmpls[1].afmt).toBe(file("production_back.html") + file("panels.html"));
    expect(c.model.css).toBe(file("card.css"));
    expect(c.model.css).toContain('font-family: "Scheherazade New"');
  });

  it("makes a production card only for ranks 1 to 1000 and orders the cards by rank", () => {
    const s = openStore(":memory:");
    const make = (lemma: string, rank: number) => {
      const n = s.createNote({ lemma, english: "x", pos: "noun", rank });
      s.approve(n.id);
      return s.getNote(n.id)!;
    };
    const notes = [make("أَ", 1000), make("بَ", 1001), make("جَ", 0), make("دَ", 0)];
    const c = collection(buildApkg(notes, read(s), options));
    const due = (lemma: string) => c.cards.filter((x) => x.guid === notes.find((n) => n.lemma === lemma)!.id).map((x) => [x.ord, x.due]);
    expect(due("أَ")).toEqual([[0, 1000], [1, 1020]]);
    expect(due("بَ")).toEqual([[0, 1001]]);
    const alone = collection(buildApkg([notes[2]], read(s), options));
    expect(due("جَ")[0][1]).toBeGreaterThan(10000);
    expect(alone.cards.map((x) => x.due)).toEqual([due("جَ")[0][1]]);
  });

  it("packs the font and exactly the audio the cards use", () => {
    const { s, id } = approvedStore();
    const c = collection(buildApkg([s.getNote(id)!], read(s), options));
    expect(Object.values(c.media).sort()).toEqual(
      [...[1, 2, 3, 4].map((n) => `${audioKey(clip(n))}.mp3`), "_ScheherazadeNew-Regular.ttf"].sort(),
    );
    const mp3 = Object.entries(c.media).find(([, name]) => name === `${audioKey(clip(1))}.mp3`)![0];
    expect(c.files.get(mp3)).toEqual(clip(1));
  });

  it("produces identical bytes for the same cards in any order", () => {
    const { s, id } = approvedStore();
    const m = s.createNote({ lemma: "قَلَم", english: "pen", pos: "noun", rank: 5 });
    s.approve(m.id);
    const a = s.getNote(id)!;
    const b = s.getNote(m.id)!;
    const one = buildApkg([a, b], read(s), options);
    expect(buildApkg([b, a], read(s), options).equals(one)).toBe(true);
    expect(buildApkg([a, b], read(s), options).equals(one)).toBe(true);
  });

  it("refuses to build an empty package", () => {
    expect(() => buildApkg([], () => Buffer.alloc(0), options)).toThrow(/no approved cards/);
  });
});
