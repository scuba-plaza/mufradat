import { describe, expect, it } from "vitest";
import type { WordRow } from "../types";
import { parseQuery } from "./parse";
import { runQuery } from "./run";

const now = new Date(2026, 10, 6, 12, 0, 0);
const at = (day: number, hour = 12) => new Date(2026, 10, day, hour, 0, 0).toISOString();

const row = (over: Partial<WordRow>): WordRow => ({
  id: null, rank: null, lemma: "", english: "", pos: "noun", gender: "", root: "", verbForm: "", status: "todo", edited: null, tags: [], ...over,
});

const rows: WordRow[] = [
  row({ rank: 1, lemma: "كِتَاب", english: "book", gender: "m", root: "ك ت ب", status: "approved", edited: at(1), id: "a" }),
  row({ rank: 2, lemma: "كَتَبَ", english: "to write", pos: "verb", verbForm: "I", root: "ك ت ب", status: "progress", edited: at(5), id: "b" }),
  row({ rank: 3, lemma: "مَدْرَسَة", english: "school", gender: "f", root: "د ر س", status: "ready", edited: at(6, 9), id: "c" }),
  row({ rank: 4, lemma: "أَنَّ", english: "that", pos: "conj" }),
  row({ rank: null, lemma: "إِنَّ", english: "indeed", pos: "particle", status: "progress", edited: at(6, 10), id: "d" }),
  row({ rank: 2500, lemma: "مَكْتَبَة", english: "library, bookshop", gender: "f", root: "ك ت ب" }),
];

const find = (text: string) => {
  const parsed = parseQuery(text);
  if ("error" in parsed) throw new Error(parsed.error);
  return runQuery(rows, parsed.query, now).map((r) => r.english);
};

describe("finding words by text", () => {
  it("returns every word, in the order given, for an empty query", () => {
    expect(find("")).toEqual(rows.map((r) => r.english));
  });

  it("matches the Arabic without its vowel marks and with another hamza", () => {
    expect(find("كتاب")).toEqual(["book", "to write"]);
    expect(find("كتب")).toEqual(["to write", "library, bookshop"]);
    expect(find("ان")).toEqual(["that", "indeed"]);
  });

  it("matches the English, and a word with a typo in it", () => {
    expect(find("school")).toEqual(["school"]);
    expect(find("scool")).toEqual(["school"]);
    expect(find("writ")).toEqual(["to write"]);
  });

  it("needs every term to match", () => {
    expect(find("book write")).toEqual([]);
    expect(find("book shop")).toEqual(["library, bookshop"]);
    expect(find("library book")).toEqual(["library, bookshop"]);
  });

  it("puts the closest match first and keeps the given order between equals", () => {
    expect(find("book")).toEqual(["book", "library, bookshop"]);
    expect(find("oo")).toEqual(["book", "school", "library, bookshop"]);
  });

  it("matches nothing for a term that is only vowel marks", () => {
    expect(find("َ")).toEqual([]);
    expect(find("٣")).toEqual([]);
  });

  it("matches a phrase only as typed", () => {
    expect(find('"to write"')).toEqual(["to write"]);
    expect(find('"to wrote"')).toEqual([]);
  });
});

describe("filtering by field", () => {
  it("keeps the words in any of the values of a field", () => {
    expect(find("status:approved")).toEqual(["book"]);
    expect(find("status:progress,ready")).toEqual(["to write", "school", "indeed"]);
    expect(find("pos:verb,conj")).toEqual(["to write", "that"]);
    expect(find("pos:particle")).toEqual(["indeed"]);
  });

  it("needs every field to match", () => {
    expect(find("pos:noun status:todo")).toEqual(["library, bookshop"]);
  });

  it("takes words out with a minus", () => {
    expect(find("-status:todo")).toEqual(["book", "to write", "school", "indeed"]);
    expect(find("pos:noun -status:todo -status:approved")).toEqual(["school"]);
  });

  it("reads rank as a number, a range, a comparison, none, or a list of them", () => {
    expect(find("rank:2")).toEqual(["to write"]);
    expect(find("rank:1-3")).toEqual(["book", "to write", "school"]);
    expect(find("rank:none")).toEqual(["indeed"]);
    expect(find("rank:1,none")).toEqual(["book", "indeed"]);
    expect(find("-rank:none rank:1-2500")).toEqual(["book", "to write", "school", "that", "library, bookshop"]);
    expect(() => find("rank:>3")).toThrow(/rank: expects/);
    expect(() => find("rank:>=4")).toThrow(/rank: expects/);
    expect(() => find("rank:<2")).toThrow(/rank: expects/);
    expect(() => find("rank:<=2")).toThrow(/rank: expects/);
  });

  it("compares a root without vowels, hamza forms or spaces", () => {
    expect(find("root:كتب")).toEqual(["book", "to write", "library, bookshop"]);
    expect(find("root:كتب,درس")).toEqual(["book", "to write", "school", "library, bookshop"]);
    expect(find('root:"ك ت ب"')).toEqual(["book", "to write", "library, bookshop"]);
  });

  it("filters by gender and verb form", () => {
    expect(find("gender:f")).toEqual(["school", "library, bookshop"]);
    expect(find("gender:m,f")).toEqual(["book", "school", "library, bookshop"]);
    expect(find("verbform:I")).toEqual(["to write"]);
    expect(find("verbform:i")).toEqual(["to write"]);
  });

  it("no longer knows the fields that were cut", () => {
    expect(() => find("missing:audio")).toThrow(/Unknown field/);
    expect(() => find("has:error")).toThrow(/Unknown field/);
    expect(() => find("example:mine")).toThrow(/Unknown field/);
  });

  it("matches a suspended row only for status:suspended", () => {
    const parked = row({ rank: 7, lemma: "\u0645\u064e\u0648\u0642\u0648\u0641", english: "parked", status: "suspended", edited: at(2), id: "s" });
    const among = [...rows, parked];
    const ask = (text: string) => {
      const parsed = parseQuery(text);
      if ("error" in parsed) throw new Error(parsed.error);
      return runQuery(among, parsed.query, now).map((r) => r.english);
    };
    expect(ask("status:suspended")).toEqual(["parked"]);
    expect(ask("status:progress,ready,approved")).toEqual(["book", "to write", "school", "indeed"]);
    expect(ask("-status:suspended")).toEqual(["book", "to write", "school", "that", "indeed", "library, bookshop"]);
  });
});

describe("filtering by when a card was last edited", () => {
  it("reads 12h, 7d and 2w as that long before now", () => {
    expect(find("edited:12h")).toEqual(["school", "indeed"]);
    expect(find("edited:2d")).toEqual(["to write", "school", "indeed"]);
    expect(find("edited:7d")).toEqual(["book", "to write", "school", "indeed"]);
    expect(find("edited:1w")).toEqual(["book", "to write", "school", "indeed"]);
    expect(find("edited:1h")).toEqual([]);
  });

  it("no longer knows a calendar date for edits", () => {
    expect(() => find("edited:2026-11-05")).toThrow(/edited: expects/);
    expect(() => find("edited:2026-11-05..")).toThrow(/edited: expects/);
    expect(() => find("edited:..2026-11-05")).toThrow(/edited: expects/);
    expect(() => find("edited:2026-11-02..2026-11-05")).toThrow(/edited: expects/);
  });

  it("finds the words never edited with none", () => {
    expect(find("edited:none")).toEqual(["that", "library, bookshop"]);
    expect(find("-edited:none")).toEqual(["book", "to write", "school", "indeed"]);
  });
});

describe("sorting", () => {
  it("sorts by rank with the words that have none last, in either direction", () => {
    expect(find("sort:rank")).toEqual(["book", "to write", "school", "that", "library, bookshop", "indeed"]);
    expect(find("sort:-rank")).toEqual(["library, bookshop", "that", "school", "to write", "book", "indeed"]);
  });

  it("sorts by the last edit with the words never edited last", () => {
    expect(find("sort:-edited")).toEqual(["indeed", "school", "to write", "book", "that", "library, bookshop"]);
    expect(find("sort:edited")).toEqual(["book", "to write", "school", "indeed", "that", "library, bookshop"]);
  });

  it("sorts the Arabic without vowel marks and the English without case", () => {
    expect(find("sort:arabic")).toEqual(["that", "indeed", "book", "to write", "school", "library, bookshop"]);
    expect(find("sort:-english")).toEqual(["to write", "that", "school", "library, bookshop", "indeed", "book"]);
  });

  it("sorts what the filters kept and puts the sort before the closeness of a text", () => {
    expect(find("pos:noun sort:-rank")).toEqual(["library, bookshop", "school", "book"]);
    expect(find("book sort:-rank")).toEqual(["library, bookshop", "book"]);
  });
});

describe("matching a tag", () => {
  const tagged = [
    row({ rank: 1, lemma: "واحد", english: "one", tags: ["lesson_1"], id: "t1" }),
    row({ rank: 2, lemma: "عشرة", english: "ten", tags: ["lesson_10", "extra"], id: "t2" }),
    row({ rank: 3, lemma: "بدون", english: "none", tags: [], id: "t3" }),
  ];
  const ask = (text: string) => {
    const parsed = parseQuery(text);
    if ("error" in parsed) throw new Error(parsed.error);
    return runQuery(tagged, parsed.query, now).map((r) => r.english);
  };

  it("matches a tag by its whole name only, with no glob", () => {
    expect(ask("tag:lesson_1")).toEqual(["one"]);
    expect(ask("tag:lesson_*")).toEqual([]);
    expect(ask("tag:LESSON_10")).toEqual(["ten"]);
    expect(ask("tag:lesson_1,extra")).toEqual(["one", "ten"]);
    expect(ask("-tag:lesson_10")).toEqual(["one", "none"]);
  });
});
