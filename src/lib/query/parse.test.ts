import { describe, expect, it } from "vitest";
import { nextSort, parseQuery, withSort } from "./parse";

const failure = (text: string) => {
  const parsed = parseQuery(text);
  return "error" in parsed ? parsed.error : null;
};

const texts = (text: string) => {
  const parsed = parseQuery(text);
  return "query" in parsed ? parsed.query.texts : null;
};

describe("reading what was typed in the bar", () => {
  it("accepts nothing, spaces and plain words", () => {
    expect(failure("")).toBeNull();
    expect(failure("   ")).toBeNull();
    expect(failure("book")).toBeNull();
  });

  it("takes each word as a term to find and keeps a quoted phrase together", () => {
    expect(texts("book  write")).toEqual([{ term: "book", phrase: false }, { term: "write", phrase: false }]);
    expect(texts('"to write" book')).toEqual([{ term: "to write", phrase: true }, { term: "book", phrase: false }]);
  });

  it("takes a term with a quote in the middle as a phrase when it holds a space", () => {
    expect(texts('foo"bar baz"qux')).toEqual([{ term: "foobar bazqux", phrase: true }]);
  });

  it("does not take a field term for text", () => {
    expect(texts("pos:noun book")).toEqual([{ term: "book", phrase: false }]);
  });
});

describe("what the bar says about a term it cannot use", () => {
  it("names a field that does not exist and lists the ones that do", () => {
    expect(failure("colour:red")).toBe(
      'Unknown field "colour". Fields: status, pos, rank, root, gender, verbform, missing, has, example, edited, tag, sort.',
    );
  });

  it("names the field and what it allows for a value it does not know", () => {
    expect(failure("status:done")).toBe('status: expects todo, progress, ready or approved, not "done".');
    expect(failure("pos:animal")).toBe(
      'pos: expects noun, verb, adj, adv, prep, conj, particle, pron, num, intj, phrase or det, not "animal".',
    );
    expect(failure("missing:colour")).toBe('missing: expects example, translation, target or audio, not "colour".');
    expect(failure("has:colour")).toBe('has: expects error, not "colour".');
    expect(failure("example:both")).toBe('example: expects mistral or mine, not "both".');
    expect(failure("gender:x")).toBe('gender: expects m, f or m+f, not "x".');
    expect(failure("verbform:XIV")).toBe(
      'verbform: expects I, II, III, IV, V, VI, VII, VIII, IX, X, XI, XII, XIII, Iq, IIq, IIIq or IVq, not "XIV".',
    );
  });

  it("says what a rank or a date has to look like", () => {
    expect(failure("rank:abc")).toBe('rank: expects a number, a range such as 1-500, >2000, <=100 or none, not "abc".');
    expect(failure("rank:500-1")).toBe('rank: expects a number, a range such as 1-500, >2000, <=100 or none, not "500-1".');
    expect(failure("edited:soon")).toBe('edited: expects 7d, 12h, 2w, a date such as 2026-11-01, a range such as 2026-11-01.. or ..2026-11-01, or none, not "soon".');
    expect(failure("edited:2026-13-40")).toContain("edited: expects");
  });

  it("asks for a value after the colon", () => {
    expect(failure("pos:")).toBe("pos: needs a value.");
    expect(failure("pos:noun,")).toBe("pos: needs a value.");
  });

  it("allows a minus only in front of a field term", () => {
    expect(failure("-book")).toBe('A minus goes in front of a field term, such as -pos:verb. To find text that starts with one, put it in quotes: "-book".');
    expect(failure('-"book"')).toContain("A minus goes in front of a field term");
    expect(failure('"-book"')).toBeNull();
    expect(failure("-pos:verb")).toBeNull();
  });

  it("says when a quote is not closed", () => {
    expect(failure('"to write')).toBe("A quote is not closed.");
    expect(failure('pos:noun "book')).toBe("A quote is not closed.");
    expect(failure('"to write"')).toBeNull();
  });

  it("reads field names and sort keys in any case, and values with a space in quotes", () => {
    expect(failure("POS:noun")).toBeNull();
    expect(failure("Sort:-RANK")).toBeNull();
    expect(failure('status:"in progress"')).toBe('status: expects todo, progress, ready or approved, not "in progress".');
    expect(failure('root:"ك ت ب"')).toBeNull();
  });

  it("refuses a date range that ends before it starts", () => {
    expect(failure("edited:2026-11-05..2026-11-01")).toContain("edited: expects");
    expect(failure("edited:2026-11-01..2026-11-05")).toBeNull();
    expect(failure("edited:2026-11-01..2026-11-01")).toBeNull();
  });

  it("allows at most 12 terms and 12 values in a field", () => {
    expect(failure(Array(12).fill("book").join(" "))).toBeNull();
    expect(failure(Array(13).fill("book").join(" "))).toBe("At most 12 terms.");
    expect(failure(`pos:${Array(12).fill("noun").join(",")}`)).toBeNull();
    expect(failure(`pos:${Array(13).fill("noun").join(",")}`)).toBe("pos: at most 12 values.");
  });

  it("allows one sort, by a known key", () => {
    expect(failure("sort:rank sort:edited")).toBe("Only one sort: is allowed.");
    expect(failure("sort:colour")).toBe('sort: expects rank, edited, arabic or english, optionally with a minus in front, not "colour".');
    expect(failure("sort:-edited")).toBeNull();
  });

  it("reports the first term it cannot use", () => {
    expect(failure("status:done pos:animal")).toBe('status: expects todo, progress, ready or approved, not "done".');
  });
});

describe("changing the sort of a query", () => {
  it("replaces the sort and keeps every other term as typed", () => {
    expect(withSort('"to write" pos:noun sort:rank', "-edited")).toBe('"to write" pos:noun sort:-edited');
  });

  it("adds a sort to a query without one and removes it for null", () => {
    expect(withSort("pos:noun", "arabic")).toBe("pos:noun sort:arabic");
    expect(withSort("pos:noun sort:arabic", null)).toBe("pos:noun");
    expect(withSort("", "rank")).toBe("sort:rank");
  });
});

describe("what a click on a column header sorts by", () => {
  const sortOf = (text: string) => {
    const parsed = parseQuery(text);
    return "query" in parsed ? parsed.query.sort : null;
  };

  it("sorts ascending first, then descending, then goes back to the default order", () => {
    expect(nextSort(sortOf(""), "arabic")).toBe("arabic");
    expect(nextSort(sortOf("sort:arabic"), "arabic")).toBe("-arabic");
    expect(nextSort(sortOf("sort:-arabic"), "arabic")).toBeNull();
  });

  it("starts ascending on a column when another column is sorted", () => {
    expect(nextSort(sortOf("sort:-english"), "arabic")).toBe("arabic");
  });
});
