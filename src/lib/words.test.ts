import { afterEach, describe, expect, it, vi } from "vitest";
import { continueFrom, continueTarget, defaultOrder, nextCardPath, openCard, wordRows, type WordRow } from "./words";
import { openStore, type Store } from "./store";

const mp3 = Buffer.from([0xff, 0xfb, 0x90, 0x00, 1, 2, 3, 4]);

function finish(s: Store, id: string) {
  const { lemma } = s.getNote(id)!;
  s.updateNote(id, { example: `هٰذَا [${lemma}] جَيِّدٌ`, exampleEn: "This is good." });
  s.setAudio(id, "word", mp3, "tts");
  s.setAudio(id, "example", mp3, "tts");
}

afterEach(() => vi.useRealTimers());

describe("the list of words", () => {
  it("has all 5000 frequency words in rank order, each one still to do without a card", () => {
    const rows = wordRows(openStore(":memory:"));
    expect(rows).toHaveLength(5000);
    expect(rows[0]).toEqual({ id: null, rank: 1, lemma: "فِي", english: "in, within", status: "todo", edited: null });
    expect(rows.map((w) => w.rank)).toEqual(rows.map((_, i) => i + 1));
  });

  it("shows what you wrote on a card instead of the Wiktionary text", () => {
    const s = openStore(":memory:");
    const id = openCard(s, { rank: 1 })!;
    s.updateNote(id, { english: "in (a place)" });
    expect(wordRows(s)[0]).toMatchObject({ id, english: "in (a place)" });
    expect(wordRows(s)[1].english).toBe("having partitive effect: of, some of, parts of, one of");
  });

  it("calls a card progress while it lacks something, ready once it lacks nothing, and approved once approved", () => {
    const s = openStore(":memory:");
    openCard(s, { rank: 1 });
    finish(s, openCard(s, { rank: 2 })!);
    const approved = openCard(s, { rank: 3 })!;
    finish(s, approved);
    expect(s.approve(approved).ok).toBe(true);
    expect(wordRows(s).slice(0, 4).map((w) => w.status)).toEqual(["progress", "ready", "approved", "todo"]);
  });

  it("calls a bare card progress although it is stored as a draft", () => {
    const s = openStore(":memory:");
    const id = openCard(s, { rank: 1 })!;
    expect(s.getNote(id)!.status).toBe("draft");
    expect(wordRows(s)[0].status).toBe("progress");
  });

  it("takes a card that lost its approval back to ready or progress", () => {
    const s = openStore(":memory:");
    const id = openCard(s, { rank: 1 })!;
    finish(s, id);
    s.approve(id);
    s.updateNote(id, { hint: "a hint" });
    expect(wordRows(s)[0].status).toBe("ready");
    s.updateNote(id, { example: "" });
    expect(wordRows(s)[0].status).toBe("progress");
  });

  it("lists the cards you made yourself after the frequency words, without a rank", () => {
    const s = openStore(":memory:");
    const id = openCard(s, { lemma: "عَلَى", pos: "verb" })!;
    const rows = wordRows(s);
    expect(rows).toHaveLength(5001);
    expect(rows[5000]).toMatchObject({ id, rank: null, lemma: "عَلَى", status: "progress" });
  });
});

describe("the order of the list when nothing is asked for", () => {
  const row = (rank: number | null, status: WordRow["status"], edited: string | null, lemma = String(rank)): WordRow => ({
    id: edited ? `card-${lemma}` : null, rank, lemma, english: "", status, edited,
  });

  it("puts cards being worked on first, latest edit on top, then words not started by rank, then approved ones by rank", () => {
    const rows = [
      row(1, "approved", "2026-01-01T00:00:00Z"),
      row(2, "todo", null),
      row(3, "progress", "2026-01-02T00:00:00Z"),
      row(4, "ready", "2026-01-03T00:00:00Z"),
      row(null, "approved", "2026-01-06T00:00:00Z", "added approved"),
      row(5, "todo", null),
      row(6, "approved", "2026-01-05T00:00:00Z"),
      row(null, "progress", "2026-01-04T00:00:00Z", "added"),
    ];
    expect(defaultOrder(rows).map((r) => r.lemma)).toEqual(["added", "4", "3", "2", "5", "1", "6", "added approved"]);
  });

  it("leaves the rows it was given as they were", () => {
    const rows = [row(2, "todo", null), row(1, "progress", "2026-01-01T00:00:00Z")];
    defaultOrder(rows);
    expect(rows.map((r) => r.lemma)).toEqual(["2", "1"]);
  });
});

describe("what Continue opens", () => {
  const row = (rank: number | null, status: WordRow["status"], edited: string | null): WordRow => ({
    id: edited ? `card-${rank}` : null, rank, lemma: String(rank), english: "", status, edited,
  });

  it("is the card being worked on that was edited last", () => {
    const rows = [row(1, "progress", "2026-01-01T00:00:00Z"), row(2, "ready", "2026-01-03T00:00:00Z"), row(3, "progress", "2026-01-02T00:00:00Z"), row(4, "todo", null)];
    expect(continueFrom(rows)).toBe(rows[1]);
  });

  it("is the most frequent word not started when nothing is being worked on", () => {
    const rows = [row(1, "approved", "2026-01-09T00:00:00Z"), row(3, "todo", null), row(2, "todo", null)];
    expect(continueFrom(rows)).toBe(rows[2]);
  });

  it("is nothing when every word is approved", () => {
    expect(continueFrom([row(1, "approved", "2026-01-01T00:00:00Z")])).toBeNull();
  });

  it("starts with the first word in a fresh workspace", () => {
    expect(continueTarget(openStore(":memory:"))).toMatchObject({ id: null, rank: 1, lemma: "فِي" });
  });

  it("follows the card you edited last, also one you made yourself", () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    const s = openStore(":memory:");
    vi.setSystemTime(new Date("2026-11-01T10:00:00Z"));
    const first = openCard(s, { rank: 1 })!;
    vi.setSystemTime(new Date("2026-11-01T10:01:00Z"));
    const added = openCard(s, { lemma: "عَلَى", pos: "verb" })!;
    expect(continueTarget(s)).toMatchObject({ id: added, rank: null });
    vi.setSystemTime(new Date("2026-11-01T10:02:00Z"));
    s.updateNote(first, { hint: "a hint" });
    expect(continueTarget(s)).toMatchObject({ id: first, rank: 1 });
  });

  it("moves on to the next word once the card is approved", () => {
    const s = openStore(":memory:");
    const first = openCard(s, { rank: 1 })!;
    finish(s, first);
    expect(s.approve(first).ok).toBe(true);
    expect(continueTarget(s)).toMatchObject({ id: null, rank: 2 });
  });

  it("leads to the card, creating the first word's card in a fresh workspace and finding it afterwards", () => {
    const s = openStore(":memory:");
    const path = nextCardPath(s);
    expect(path).toBe(`/cards/${s.findNote("فِي", "prep")}`);
    expect(nextCardPath(s)).toBe(path);
  });
});

describe("opening a word", () => {
  it("creates the card the first time and finds it afterwards", () => {
    const s = openStore(":memory:");
    const id = openCard(s, { rank: 3 })!;
    expect(s.getNote(id)).toMatchObject({ lemma: "عَلَى", pos: "prep", rank: 3 });
    expect(openCard(s, { rank: 3 })).toBe(id);
    expect(s.allNotes()).toHaveLength(1);
  });

  it("opens a Wiktionary entry that is not in the list as an unranked card", () => {
    const s = openStore(":memory:");
    const id = openCard(s, { lemma: "عَلَى", pos: "verb" })!;
    expect(s.getNote(id)).toMatchObject({ pos: "verb", rank: 0 });
  });

  it("opens the entry that was chosen when other entries share its letters and part of speech", () => {
    const s = openStore(":memory:");
    const chosen = [
      { lemma: "إِن", pos: "conj" },
      { lemma: "أَنْ", pos: "conj" },
      { lemma: "آنَ", pos: "verb" },
    ];
    const ids = chosen.map((target) => openCard(s, target)!);
    expect(ids.map((id) => s.getNote(id)!.lemma)).toEqual(chosen.map((target) => target.lemma));
    expect(new Set(ids).size).toBe(chosen.length);
  });

  it("says so when there is no such word", () => {
    const s = openStore(":memory:");
    expect(openCard(s, { rank: 5001 })).toBeNull();
    expect(openCard(s, { lemma: "zzz", pos: "noun" })).toBeNull();
    expect(openCard(s, { lemma: "على", pos: "nonsense" })).toBeNull();
    expect(s.allNotes()).toHaveLength(0);
  });
});
