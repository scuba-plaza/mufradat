import { afterEach, describe, expect, it, vi } from "vitest";
import { continueFrom, continueTarget, defaultOrder, cardPath, neighbour, nextCardPath, openCard, rowHref, view, wordRows } from "./words";
import { openStore, type Store } from "./store";
import { MISSING_KINDS, type WordRow } from "./types";

const mp3 = Buffer.from([0xff, 0xfb, 0x90, 0x00, 1, 2, 3, 4]);

function finish(s: Store, id: string) {
  const { lemma } = s.getNote(id)!;
  s.updateNote(id, { example: `هٰذَا [${lemma}] جَيِّدٌ`, exampleEn: "This is good." });
  s.setAudio(id, "word", mp3, "tts");
  s.setAudio(id, "example", mp3, "tts");
}

const blank: WordRow = {
  id: null, rank: null, lemma: "", english: "", pos: "noun", gender: "", root: "", verbForm: "", status: "todo",
  missing: [...MISSING_KINDS], error: false, exampleBy: "", edited: null,
};

afterEach(() => vi.useRealTimers());

describe("the list of words", () => {
  it("has all 5000 frequency words in rank order, each one still to do without a card", () => {
    const rows = wordRows(openStore(":memory:"));
    expect(rows).toHaveLength(5000);
    expect(rows[0]).toEqual({ ...blank, rank: 1, lemma: "فِي", english: "in, within", pos: "prep" });
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

describe("what a row says about a word", () => {
  it("tells a frequency word that has no card by the part of speech its card will have, root and verb form, and says it still lacks everything", () => {
    const rows = wordRows(openStore(":memory:"));
    expect(rows[5]).toEqual({ ...blank, rank: 6, lemma: "كَانَ", english: "to be", pos: "verb", verbForm: "I" });
    expect(rows[67]).toMatchObject({ pos: "noun", gender: "m", root: "ء ح د" });
  });

  it("tells a card by what it has and what it lacks", () => {
    const s = openStore(":memory:");
    const id = openCard(s, { rank: 6 })!;
    s.setAudio(id, "word", mp3, "tts");
    expect(wordRows(s)[5]).toEqual({
      ...blank, id, rank: 6, lemma: "كَانَ", english: "to be", pos: "verb", verbForm: "I", status: "progress",
      missing: ["example", "audio"], edited: s.getNote(id)!.updatedAt,
    });
  });

  it("tells who wrote the example and whether a failure is still shown", () => {
    const s = openStore(":memory:");
    const [mine, generated, failed, clip] = [1, 2, 3, 4].map((rank) => openCard(s, { rank })!);
    s.updateNote(mine, { example: "هٰذَا [فِي] الْبَيْتِ" });
    s.applyGeneratedExample(generated, "هٰذَا [مِنْ] الْبَيْتِ", "this is from the house");
    s.setExampleError(failed, "Mistral is busy (429).");
    s.setAudioError(clip, "word", "Google rejected the credentials (401).");
    const rows = wordRows(s).slice(0, 5);
    expect(rows.map((r) => r.exampleBy)).toEqual(["user", "mistral", "", "", ""]);
    expect(rows.map((r) => r.error)).toEqual([false, false, true, true, false]);
  });
});

describe("the order of the list when nothing is asked for", () => {
  const row = (rank: number | null, status: WordRow["status"], edited: string | null, lemma = String(rank)): WordRow => ({
    ...blank, id: edited ? `card-${lemma}` : null, rank, lemma, status, edited,
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
    ...blank, id: edited ? `card-${rank}` : null, rank, lemma: String(rank), status, edited,
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

describe("the words a query asks for", () => {
  const now = new Date("2026-11-06T12:00:00Z");
  const lemmas = (text: string, s: Store = openStore(":memory:")) => view(s, text, now).rows.map((r) => r.lemma);

  it("is every word in the default order for an empty query, and has no error", () => {
    const s = openStore(":memory:");
    openCard(s, { rank: 3 });
    const { rows, error } = view(s, "", now);
    expect(error).toBeNull();
    expect(rows).toHaveLength(5000);
    expect(rows[0]).toMatchObject({ rank: 3, status: "progress" });
    expect(rows[1]).toMatchObject({ rank: 1, status: "todo" });
  });

  it("filters the frequency words and the cards alike", () => {
    const s = openStore(":memory:");
    finish(s, openCard(s, { rank: 1 })!);
    openCard(s, { rank: 2 });
    expect(lemmas("status:ready", s)).toEqual(["فِي"]);
    expect(view(s, "status:progress", now).rows.map((r) => r.rank)).toEqual([2]);
    expect(view(s, "status:todo rank:<5", now).rows.map((r) => r.rank)).toEqual([3, 4]);
    expect(view(s, "pos:prep rank:<10", now).rows.map((r) => r.rank)).toEqual([1, 2, 3, 9]);
  });

  it("lists the words of a query in the order of the frequency list, whatever you worked on last", () => {
    const s = openStore(":memory:");
    openCard(s, { rank: 9 });
    expect(view(s, "", now).rows[0]).toMatchObject({ rank: 9 });
    expect(view(s, "pos:prep rank:<12", now).rows.map((r) => r.rank)).toEqual([1, 2, 3, 9]);
    expect(view(s, "status:progress", now).rows.map((r) => r.rank)).toEqual([9]);
  });

  it("reports the problem with the query and shows no rows", () => {
    expect(view(openStore(":memory:"), "pos:animal", now)).toEqual({
      rows: [],
      sort: null,
      error: 'pos: expects noun, verb, adj, adv, prep, conj, particle, pron, num, intj, phrase or det, not "animal".',
    });
  });

  it("hands on the sort it read from the query", () => {
    const s = openStore(":memory:");
    expect(view(s, "sort:-rank", now).sort).toEqual({ key: "rank", descending: true });
    expect(view(s, "pos:noun", now).sort).toBeNull();
    expect(view(s, "pos:animal", now).sort).toBeNull();
  });

  it("finds a word in Wiktionary that is not in the list when the query has text, once", () => {
    const s = openStore(":memory:");
    const found = view(s, "على", now).rows.filter((r) => r.lemma === "عَلَى");
    expect(found).toEqual([
      { ...blank, rank: 3, lemma: "عَلَى", english: "on, over", pos: "prep" },
      { ...blank, lemma: "عَلَى", english: "to climb", pos: "verb", verbForm: "I" },
    ]);
  });

  it("does not look in Wiktionary when the query has no text", () => {
    expect(view(openStore(":memory:"), "status:todo pos:verb", now).rows.every((r) => r.rank !== null)).toBe(true);
  });

  it("shows the card instead of the Wiktionary entry once it was opened", () => {
    const s = openStore(":memory:");
    const id = openCard(s, { lemma: "عَلَى", pos: "verb" })!;
    const found = view(s, "على", now).rows.filter((r) => r.lemma === "عَلَى");
    expect(found).toHaveLength(2);
    expect(found.find((r) => r.rank === null)).toMatchObject({ id, pos: "verb", status: "progress" });
  });

  it("finds every entry whose letters match, each as its own row", () => {
    expect(lemmas("إِن")).toEqual(expect.arrayContaining(["إِن", "أَنَّ", "أَنْ", "إِنَّ", "آن"]));
  });

  it("evaluates the filters on Wiktionary entries as words not started", () => {
    const s = openStore(":memory:");
    expect(view(s, "على status:approved", now).rows).toEqual([]);
    const verbs = view(s, "على verbform:I rank:none", now).rows;
    expect(verbs.map((r) => r.english)).toContain("to climb");
    expect(verbs.every((r) => r.verbForm === "I" && r.rank === null && r.status === "todo")).toBe(true);
  });
});

describe("where a row leads", () => {
  it("opens the card of a word that has one", () => {
    expect(rowHref({ ...blank, id: "abc", rank: 7 })).toBe("/cards/abc");
  });

  it("opens a frequency word by its rank, creating the card", () => {
    expect(rowHref({ ...blank, rank: 7 })).toBe("/open?rank=7");
  });

  it("opens a Wiktionary entry by its lemma and part of speech, written so that they read back unchanged", () => {
    const href = rowHref({ ...blank, lemma: "عَلَى", pos: "verb" });
    const url = new URL(href, "http://localhost");
    expect(url.pathname).toBe("/open");
    expect(url.searchParams.get("lemma")).toBe("عَلَى");
    expect(url.searchParams.get("pos")).toBe("verb");
  });
});

describe("where a row leads when the list came from a query", () => {
  it("puts the query on the address of a card, so that the card knows the list it was opened from", () => {
    expect(cardPath("abc")).toBe("/cards/abc");
    expect(cardPath("abc", "")).toBe("/cards/abc");
    expect(cardPath("abc", "pos:noun")).toBe("/cards/abc?q=pos%3Anoun");
    expect(rowHref({ ...blank, id: "abc" }, "pos:noun")).toBe("/cards/abc?q=pos%3Anoun");
  });

  it("hands the query on through the page that creates the card", () => {
    const query = '"to write" sort:-rank';
    const byRank = new URL(rowHref({ ...blank, rank: 7 }, query), "http://localhost");
    expect([byRank.pathname, byRank.searchParams.get("rank"), byRank.searchParams.get("q")]).toEqual(["/open", "7", query]);
    const byLemma = new URL(rowHref({ ...blank, lemma: "عَلَى", pos: "verb" }, query), "http://localhost");
    expect([byLemma.searchParams.get("lemma"), byLemma.searchParams.get("pos"), byLemma.searchParams.get("q")]).toEqual(["عَلَى", "verb", query]);
  });
});

describe("the word an arrow leads to", () => {
  const now = new Date("2026-11-06T12:00:00Z");

  it("is the next or the previous word of the frequency list when the card came from no query", () => {
    const s = openStore(":memory:");
    const [a, b, c] = [3, 4, 5].map((rank) => openCard(s, { rank })!);
    expect(neighbour(s, b, "", "next", now)).toMatchObject({ id: c, rank: 5 });
    expect(neighbour(s, b, "", "previous", now)).toMatchObject({ id: a, rank: 3 });
  });

  it("is a word without a card when its neighbour has none", () => {
    const s = openStore(":memory:");
    const id = openCard(s, { rank: 10 })!;
    expect(neighbour(s, id, "", "next", now)).toMatchObject({ id: null, rank: 11 });
    expect(neighbour(s, id, "", "previous", now)).toMatchObject({ id: null, rank: 9 });
  });

  it("is nothing past the ends of the list", () => {
    const s = openStore(":memory:");
    const [first, last] = [1, 5000].map((rank) => openCard(s, { rank })!);
    expect(neighbour(s, first, "", "previous", now)).toBeNull();
    expect(neighbour(s, last, "", "next", now)).toBeNull();
  });

  it("is nothing for a card made outside the list when there is no query", () => {
    const s = openStore(":memory:");
    const id = openCard(s, { lemma: "عَلَى", pos: "verb" })!;
    expect(neighbour(s, id, "", "next", now)).toBeNull();
    expect(neighbour(s, id, "", "previous", now)).toBeNull();
  });

  it("follows the list of the query the card was opened from", () => {
    const s = openStore(":memory:");
    const [a, b, c] = [1, 2, 3].map((rank) => openCard(s, { rank })!);
    expect(neighbour(s, b, "pos:prep rank:<12", "next", now)).toMatchObject({ id: c, rank: 3 });
    expect(neighbour(s, b, "pos:prep rank:<12", "previous", now)).toMatchObject({ id: a, rank: 1 });
  });

  it("keeps moving forward through a list when every step creates a card", () => {
    const s = openStore(":memory:");
    const query = "pos:noun rank:<60";
    let at = openCard(s, { rank: 36 })!;
    const seen = [36];
    for (let step = 0; step < 5; step++) {
      const next = neighbour(s, at, query, "next", now)!;
      seen.push(next.rank!);
      at = next.id ?? openCard(s, { rank: next.rank! })!;
    }
    expect(seen).toEqual([...seen].sort((x, y) => x - y));
    expect(new Set(seen).size).toBe(seen.length);
  });

  it("goes to the top of the list when the card no longer belongs to it, and has nothing before it", () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    const s = openStore(":memory:");
    vi.setSystemTime(new Date("2026-11-01T10:00:00Z"));
    openCard(s, { rank: 1 });
    vi.setSystemTime(new Date("2026-11-01T10:01:00Z"));
    const second = openCard(s, { rank: 2 })!;
    vi.setSystemTime(new Date("2026-11-01T10:02:00Z"));
    const done = openCard(s, { rank: 3 })!;
    finish(s, done);
    const query = "status:progress sort:-edited";
    expect(neighbour(s, done, query, "next", now)).toMatchObject({ id: second });
    expect(neighbour(s, done, query, "previous", now)).toBeNull();
  });

  it("is nothing for a query it cannot read", () => {
    const s = openStore(":memory:");
    const id = openCard(s, { rank: 3 })!;
    expect(neighbour(s, id, "pos:animal", "next", now)).toBeNull();
  });

  it("resumes at the place of a word that has just been opened from a list of the words not started", () => {
    const s = openStore(":memory:");
    const id = openCard(s, { rank: 5 })!;
    expect(neighbour(s, id, "status:todo", "next", now)).toMatchObject({ rank: 6 });
    expect(neighbour(s, id, "status:todo", "previous", now)).toMatchObject({ rank: 4 });
  });

  it("keeps a Wiktionary entry in its place in the list once it was opened, so that its neighbours stay its neighbours", () => {
    const s = openStore(":memory:");
    const query = "على rank:none";
    const lemmas = () => view(s, query, now).rows.map((r) => r.lemma);
    const before = lemmas();
    expect(before.length).toBeGreaterThan(3);
    const third = view(s, query, now).rows[2];
    const id = openCard(s, { lemma: third.lemma, pos: third.pos })!;
    expect(lemmas()).toEqual(before);
    expect(neighbour(s, id, query, "previous", now)).toMatchObject({ lemma: before[1] });
    expect(neighbour(s, id, query, "next", now)).toMatchObject({ lemma: before[3] });
  });
});
