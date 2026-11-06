import { describe, expect, it } from "vitest";
import type { WordRow } from "../types";
import { parseQuery } from "./parse";
import { runQuery } from "./run";

const now = new Date(2026, 10, 6, 12, 0, 0);
const at = (day: number, hour = 12) => new Date(2026, 10, day, hour, 0, 0).toISOString();

const row = (over: Partial<WordRow>): WordRow => ({
  id: null, rank: null, lemma: "", english: "", pos: "noun", gender: "", root: "", verbForm: "", status: "todo",
  missing: ["example", "translation", "target", "audio"], error: false, exampleBy: "", edited: null, ...over,
});

const rows: WordRow[] = [
  row({ rank: 1, lemma: "كِتَاب", english: "book", gender: "m", root: "ك ت ب", status: "approved", missing: [], exampleBy: "mistral", edited: at(1), id: "a" }),
  row({ rank: 2, lemma: "كَتَبَ", english: "to write", pos: "verb", verbForm: "I", root: "ك ت ب", status: "progress", missing: ["audio"], error: true, exampleBy: "user", edited: at(5), id: "b" }),
  row({ rank: 3, lemma: "مَدْرَسَة", english: "school", gender: "f", root: "د ر س", status: "ready", missing: [], exampleBy: "mistral", edited: at(6, 9), id: "c" }),
  row({ rank: 4, lemma: "أَنَّ", english: "that", pos: "conj" }),
  row({ rank: null, lemma: "إِنَّ", english: "indeed", pos: "particle", status: "progress", missing: ["example", "translation", "target", "audio"], edited: at(6, 10), id: "d" }),
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
    expect(find("rank:>3")).toEqual(["that", "library, bookshop"]);
    expect(find("rank:>=4")).toEqual(["that", "library, bookshop"]);
    expect(find("rank:<2")).toEqual(["book"]);
    expect(find("rank:<=2")).toEqual(["book", "to write"]);
    expect(find("rank:none")).toEqual(["indeed"]);
    expect(find("rank:1,none")).toEqual(["book", "indeed"]);
    expect(find("-rank:none rank:>0")).toEqual(["book", "to write", "school", "that", "library, bookshop"]);
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

  it("filters by what a card lacks, a word with no card lacking all of it", () => {
    expect(find("missing:audio")).toEqual(["to write", "that", "indeed", "library, bookshop"]);
    expect(find("missing:example status:progress")).toEqual(["indeed"]);
    expect(find("-missing:audio")).toEqual(["book", "school"]);
  });

  it("filters by an error that is still shown", () => {
    expect(find("has:error")).toEqual(["to write"]);
    expect(find("-has:error status:progress")).toEqual(["indeed"]);
  });

  it("filters by who wrote the example", () => {
    expect(find("example:mistral")).toEqual(["book", "school"]);
    expect(find("example:mine")).toEqual(["to write"]);
    expect(find("-example:mistral -example:mine status:progress")).toEqual(["indeed"]);
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

  it("reads a date as that day and a range as from or until a day, both included", () => {
    expect(find("edited:2026-11-05")).toEqual(["to write"]);
    expect(find("edited:2026-11-05..")).toEqual(["to write", "school", "indeed"]);
    expect(find("edited:..2026-11-05")).toEqual(["book", "to write"]);
    expect(find("edited:2026-11-02..2026-11-05")).toEqual(["to write"]);
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
