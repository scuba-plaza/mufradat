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
      'Unknown field "colour". Fields: status, pos, rank, root, gender, verbform, edited, tag, sort.',
    );
  });

  it("names the field and what it allows for a value it does not know", () => {
    expect(failure("status:done")).toBe('status: expects todo, progress, ready, approved or suspended, not "done".');
    expect(failure("pos:animal")).toBe(
      'pos: expects noun, verb, adj, adv, prep, conj, particle, pron, num, intj, phrase or det, not "animal".',
    );
    expect(failure("missing:colour")).toBe('Unknown field "missing". Fields: status, pos, rank, root, gender, verbform, edited, tag, sort.');
    expect(failure("has:colour")).toBe('Unknown field "has". Fields: status, pos, rank, root, gender, verbform, edited, tag, sort.');
    expect(failure("example:both")).toBe('Unknown field "example". Fields: status, pos, rank, root, gender, verbform, edited, tag, sort.');
    expect(failure("gender:x")).toBe('gender: expects m, f or m+f, not "x".');
    expect(failure("verbform:XIV")).toBe(
      'verbform: expects I, II, III, IV, V, VI, VII, VIII, IX, X, XI, XII, XIII, Iq, IIq, IIIq or IVq, not "XIV".',
    );
  });

  it("says what a rank or a span has to look like", () => {
    expect(failure("rank:abc")).toBe('rank: expects a number, a range such as 1-500 or none, not "abc".');
    expect(failure("rank:500-1")).toBe('rank: expects a number, a range such as 1-500 or none, not "500-1".');
    expect(failure("rank:>2000")).toBe('rank: expects a number, a range such as 1-500 or none, not ">2000".');
    expect(failure("edited:soon")).toBe('edited: expects a span such as 7d, 12h or 2w, or none, not "soon".');
    expect(failure("edited:2026-13-40")).toBe('edited: expects a span such as 7d, 12h or 2w, or none, not "2026-13-40".');
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
    expect(failure('status:"in progress"')).toBe('status: expects todo, progress, ready, approved or suspended, not "in progress".');
    expect(failure('root:"ك ت ب"')).toBeNull();
  });

  it("refuses a calendar date, which is no longer a way to ask about edits", () => {
    expect(failure("edited:2026-11-01")).toContain("edited: expects");
    expect(failure("edited:2026-11-01..")).toContain("edited: expects");
    expect(failure("edited:..2026-11-01")).toContain("edited: expects");
    expect(failure("edited:2026-11-01..2026-11-05")).toContain("edited: expects");
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
    expect(failure("status:done pos:animal")).toBe('status: expects todo, progress, ready, approved or suspended, not "done".');
  });
});

describe("a query that asks for suspended words", () => {
  const flag = (text: string) => {
    const parsed = parseQuery(text);
    return "query" in parsed ? parsed.query.suspended : null;
  };

  it("is marked when the query has a positive status:suspended term", () => {
    expect(flag("status:suspended")).toBe(true);
    expect(flag("status:suspended,progress")).toBe(true);
    expect(flag("tag:x status:suspended")).toBe(true);
  });

  it("is not marked by a minus or by the other statuses", () => {
    expect(flag("")).toBe(false);
    expect(flag("status:progress,ready")).toBe(false);
    expect(flag("-status:suspended")).toBe(false);
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

describe("the one way in, re:", () => {
  it("takes re: in front of free text as a pattern, not as text", () => {
    expect(failure("re:kitab.*")).toBeNull();
    expect(texts("re:kitab.*")).toEqual([]);
  });

  it("takes re: after a field that holds free-form text", () => {
    expect(failure("tag:re:lesson_[0-9]+")).toBeNull();
    expect(failure("root:re:ك.تب")).toBeNull();
  });

  it("names the pattern's own error under the field it was written in", () => {
    expect(failure("re:")).toBe("re: needs a pattern.");
    expect(failure("re:[a")).toContain("not closed");
    expect(failure("tag:re:[a")).toContain("not closed");
    expect(failure("tag:re:")).toContain("needs a pattern");
  });

  it("refuses re: on a field that takes fixed values", () => {
    expect(failure("status:re:.*")).toBe("status: takes fixed values, not a pattern.");
    expect(failure("pos:re:.*")).toBe("pos: takes fixed values, not a pattern.");
    expect(failure("rank:re:.*")).toBe("rank: takes fixed values, not a pattern.");
    expect(failure("edited:re:.*")).toBe("edited: takes fixed values, not a pattern.");
    expect(failure("sort:re:.*")).toContain("sort:");
  });

  it("keeps a comma in a re: value inside the pattern", () => {
    expect(failure("tag:re:a{1,2}")).toBeNull();
  });
});

describe("what the bar says about a pattern among other values", () => {
  it("names a bad pattern once, under the field it was written in", () => {
    expect(failure("tag:a,re:[a")).toBe("tag:re: A class in [] is not closed.");
    expect(failure("tag:a,re:")).toBe("tag:re: needs a pattern.");
    expect(failure("pos:noun,re:x")).toBe("pos: takes fixed values, not a pattern.");
  });

  it("reads re: in any letter case", () => {
    expect(failure("RE:[a")).toBe("re: A class in [] is not closed.");
    expect(failure("tag:RE:[a")).toBe("tag:re: A class in [] is not closed.");
    expect(failure("status:Re:.*")).toBe("status: takes fixed values, not a pattern.");
  });

  it("says a pattern of free text cannot be negated, and a field's can", () => {
    expect(failure("-re:x")).toBe("re: cannot be negated; put the minus before a field, as in -tag:re:lesson_.*.");
    expect(failure("-tag:re:x")).toBeNull();
  });

  it("names a pattern that is too big instead of failing", () => {
    expect(failure("re:((a{100,100}){100,100}){100,100}")).toBe("re: The pattern is too big to search with.");
    expect(failure("tag:re:((a{100,100}){100,100}){100,100}")).toBe("tag:re: The pattern is too big to search with.");
  });
});
