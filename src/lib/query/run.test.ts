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

const findIn = (among: WordRow[], text: string) => {
  const parsed = parseQuery(text);
  if ("error" in parsed) throw new Error(parsed.error);
  return runQuery(among, parsed.query, now).map((r) => r.english);
};

const find = (text: string) => findIn(rows, text);

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

  it("reads rank as a number, a range, none, or a list of them", () => {
    expect(find("rank:2")).toEqual(["to write"]);
    expect(find("rank:1-3")).toEqual(["book", "to write", "school"]);
    expect(find("rank:none")).toEqual(["indeed"]);
    expect(find("rank:1,none")).toEqual(["book", "indeed"]);
    expect(find("-rank:none rank:1-2500")).toEqual(["book", "to write", "school", "that", "library, bookshop"]);
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

  it("matches a suspended row only for status:suspended", () => {
    const parked = row({ rank: 7, lemma: "\u0645\u064e\u0648\u0642\u0648\u0641", english: "parked", status: "suspended", edited: at(2), id: "s" });
    const ask = (text: string) => findIn([...rows, parked], text);
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
  const ask = (text: string) => findIn(tagged, text);

  it("matches a tag by its whole name only, with no glob", () => {
    expect(ask("tag:lesson_1")).toEqual(["one"]);
    expect(ask("tag:lesson_*")).toEqual([]);
    expect(ask("tag:LESSON_10")).toEqual(["ten"]);
    expect(ask("tag:lesson_1,extra")).toEqual(["one", "ten"]);
    expect(ask("-tag:lesson_10")).toEqual(["one", "none"]);
  });
});

describe("matching with a pattern", () => {
  it("finds free text with re: against the arabic and the english, folded", () => {
    expect(find("re:^book$")).toEqual(["book"]);
    expect(find("re:writ(e|en|ing)")).toEqual(["to write"]);
    expect(find("re:كتاب")).toEqual(["book"]);
    expect(find("re:ك.*ب")).toEqual(["book", "to write", "library, bookshop"]);
    expect(find("re:librar.")).toEqual(["library, bookshop"]);
  });

  it("keeps the order of the list for a pattern, and ranks only fuzzy text", () => {
    expect(find("re:o")).toEqual(["book", "to write", "school", "library, bookshop"]);
    expect(find("re:^book$ book")).toEqual(["book"]);
  });

  it("matches tags and roots with re:", () => {
    const tagged = [
      row({ rank: 1, lemma: "واحد", english: "one", tags: ["lesson_1"], id: "t1" }),
      row({ rank: 2, lemma: "عشرة", english: "ten", tags: ["lesson_10"], id: "t2" }),
      row({ rank: 3, lemma: "مدرسة", english: "school", root: "د ر س", id: "t3" }),
    ];
      const ask = (text: string) => findIn(tagged, text);
    expect(ask("tag:re:lesson_[0-9]+")).toEqual(["one", "ten"]);
    expect(ask("tag:re:^lesson_1$")).toEqual(["one"]);
    expect(ask("-tag:re:.*")).toEqual(["school"]);
    expect(ask("root:re:.ر.")).toEqual(["school"]);
  });
});

describe("a pattern is folded like the text it looks in", () => {
  it("finds what was typed with a taa marbuta, a hamza, vowel marks or capitals", () => {
    expect(find("re:مدرسة")).toEqual(["school"]);
    expect(find("re:مَدْرَسَة")).toEqual(["school"]);
    expect(find("re:^أن$")).toEqual(["that", "indeed"]);
    expect(find("re:SCHOOL")).toEqual(["school"]);
    expect(find("re:[A-C]ook")).toEqual(["book", "library, bookshop"]);
    expect(find("re:\\D\\Dook")).toEqual(["library, bookshop"]);
  });

  it("keeps the capital of an escape and the name of a property, which mean something as typed", () => {
    expect(find("re:\\bbook\\b")).toEqual(["book"]);
    expect(find("re:\\Book")).toEqual(["book", "library, bookshop"]);
    expect(find("re:^\\p{Latin}+$")).toEqual(["book", "school", "that", "indeed"]);
    expect(find("re:^\\P{Latin}+$ ك")).toEqual(["book", "to write", "library, bookshop"]);
    expect(find("re:\\x{643}\\x{62A}")).toEqual(["book", "to write", "library, bookshop"]);
  });

  it("folds an escape for a character as it folds the character itself", () => {
    expect(find("re:^\\u0623\\u0646$")).toEqual(["that", "indeed"]);
    expect(find("re:^\\x{623}\\x{646}$")).toEqual(["that", "indeed"]);
    expect(find("re:^\\x{0623}\\x{0646}$")).toEqual(["that", "indeed"]);
    expect(find("re:^\\x42OOK$")).toEqual(["book"]);
    expect(find("re:^\\x{42}ook$")).toEqual(["book"]);
    expect(find("re:\\u0643\\u0650\\u062a\\u064e\\u0627\\u0628")).toEqual(["book"]);
    expect(find("re:\\u0643\\u064e\\u062a\\u064e\\u0628")).toEqual(["to write", "library, bookshop"]);
    expect(find("re:^[\\x{621}-\\x{64a}]+$ ك")).toEqual(["book", "to write", "library, bookshop"]);
    expect(find("re:^[\\u0627-\\u064a]{2}$")).toEqual(["that", "indeed"]);
  });

  it("finds no capital letter, since the text is folded to small letters before it is looked in", () => {
    expect(find("re:\\p{Lu}")).toEqual([]);
    expect(find("re:^\\P{Lu}+$ school")).toEqual(["school"]);
  });

  it("folds a pattern for a tag or a root the same way", () => {
    const folded = [
      row({ rank: 1, english: "eat", root: "أ ك ل", tags: ["Lesson_1"], id: "f1" }),
      row({ rank: 2, english: "other", root: "ك ت ب", tags: ["extra"], id: "f2" }),
    ];
      const ask = (text: string) => findIn(folded, text);
    expect(ask("tag:re:Lesson_[0-9]")).toEqual(["eat"]);
    expect(ask("root:re:^أ.ل$")).toEqual(["eat"]);
    expect(ask("root:re:^ك.َ?ب$")).toEqual(["other"]);
  });
});
