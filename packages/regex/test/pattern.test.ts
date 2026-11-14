import { describe, expect, it } from "vitest";
import { compilePattern } from "../src";

const finds = (pattern: string, text: string): boolean | null => {
  const compiled = compilePattern(pattern);
  if (typeof compiled === "string") return null;
  return compiled.matches(text);
};

const error = (pattern: string): string | null => {
  const compiled = compilePattern(pattern);
  return typeof compiled === "string" ? compiled : null;
};

describe("a pattern", () => {
  it("finds a literal anywhere in the text", () => {
    expect(finds("kitab", "kitab")).toBe(true);
    expect(finds("kit", "kitab")).toBe(true);
    expect(finds("tab", "kitab")).toBe(true);
    expect(finds("kitab", "the book kitab")).toBe(true);
    expect(finds("kitab", "kitbah")).toBe(false);
    expect(finds("kitab", "")).toBe(false);
  });
});

describe("escapes and the any character", () => {
  it("takes an escaped metacharacter as a literal", () => {
    expect(finds("a\\*b", "a*b")).toBe(true);
    expect(finds("a\\*b", "aaxbb")).toBe(false);
    expect(finds("\\{1,2\\}", "{1,2}")).toBe(true);
    expect(finds("a\\\\b", "a\\b")).toBe(true);
  });

  it("takes a dot as any one character", () => {
    expect(finds("k.b", "ktb")).toBe(true);
    expect(finds("k.b", "kitb")).toBe(false);
    expect(finds("k.b", "kb")).toBe(false);
    expect(finds("ك.ب", "كتب")).toBe(true);
  });
});

describe("perl character classes", () => {
  it("takes \\d as any digit, including arabic ones", () => {
    expect(finds("lesson_\\d", "lesson_5")).toBe(true);
    expect(finds("lesson_\\d", "lesson_٥")).toBe(true);
    expect(finds("lesson_\\d", "lesson_a")).toBe(false);
  });

  it("takes \\w as any letter or digit, arabic included", () => {
    expect(finds("\\w\\w\\w", "كتb")).toBe(true);
    expect(finds("\\w", "-")).toBe(false);
    expect(finds("\\w", "1")).toBe(true);
  });

  it("takes \\s as whitespace", () => {
    expect(finds("a\\sb", "a b")).toBe(true);
    expect(finds("a\\sb", "a-b")).toBe(false);
  });

  it("takes the capital forms as the opposite", () => {
    expect(finds("\\D", "5")).toBe(false);
    expect(finds("\\D", "a")).toBe(true);
    expect(finds("\\W", "a")).toBe(false);
    expect(finds("\\W", "-")).toBe(true);
    expect(finds("\\S", " ")).toBe(false);
    expect(finds("\\S", "a")).toBe(true);
  });
});

describe("custom character classes", () => {
  it("matches one of the named characters or ranges", () => {
    expect(finds("lesson_[1-3]", "lesson_2")).toBe(true);
    expect(finds("lesson_[1-3]", "lesson_4")).toBe(false);
    expect(finds("[a-d2-6]x", "5x")).toBe(true);
    expect(finds("[a-d2-6]x", "ex")).toBe(false);
    expect(finds("[a-c]+", "abca")).toBe(true);
  });

  it("matches arabic ranges", () => {
    expect(finds("[\u0627-\u064a]+", "\u0643\u062a\u0627\u0628")).toBe(true);
    expect(finds("[\u0627-\u0628]+", "\u0643")).toBe(false);
  });

  it("takes a caret as negation, not a literal", () => {
    expect(finds("[^a-z]_", "1_")).toBe(true);
    expect(finds("[^a-z]_", "b_")).toBe(false);
    expect(finds("[a^]b", "^b")).toBe(true);
  });

  it("allows escapes and perl classes inside", () => {
    expect(finds("[\\d_]+", "1_2")).toBe(true);
    expect(finds("[\\d_]+", "a")).toBe(false);
    expect(finds("[\\]]", "]")).toBe(true);
  });

  it("rejects classes it cannot read", () => {
    expect(error("[a")).toMatch(/not closed/);
    expect(error("[z-a]")).toMatch(/range/);
    expect(error("[]")).toMatch(/empty/);
  });
});

describe("quantifiers", () => {
  it("takes ? * + as zero-or-one, any, at-least-one", () => {
    expect(finds("colou?r", "color")).toBe(true);
    expect(finds("colou?r", "colour")).toBe(true);
    expect(finds("colou?r", "colouur")).toBe(false);
    expect(finds("ab*c", "ac")).toBe(true);
    expect(finds("ab*c", "abbbc")).toBe(true);
    expect(finds("ab+c", "ac")).toBe(false);
    expect(finds("ab+c", "abc")).toBe(true);
  });

  it("takes {1,2}, {,6} and {9,} as counted ranges", () => {
    expect(finds("^a{1,2}b$", "ab")).toBe(true);
    expect(finds("^a{1,2}b$", "aab")).toBe(true);
    expect(finds("^a{1,2}b$", "aaab")).toBe(false);
    expect(finds("^x{,2}y$", "y")).toBe(true);
    expect(finds("^x{,2}y$", "xxy")).toBe(true);
    expect(finds("^x{,2}y$", "xxxy")).toBe(false);
    expect(finds("a{9,}", "a".repeat(9))).toBe(true);
    expect(finds("a{9,}", "a".repeat(8))).toBe(false);
  });

  it("takes a brace that does not hold a quantifier as text", () => {
    expect(finds("a{2", "a{2")).toBe(true);
    expect(finds("a{2}", "a{2}")).toBe(true);
  });

  it("rejects quantifiers it cannot read", () => {
    expect(error("*a")).toMatch(/nothing before it/);
    expect(error("a{2,1}")).toMatch(/smaller to the larger/);
    expect(error("a{101,}")).toMatch(/more than 100/);
    expect(error("a++")).toMatch(/cannot read|quantifier/);
  });
});

describe("alternation and groups", () => {
  it("takes a|b as either side, with or without a group", () => {
    expect(finds("kitab|katab", "katab")).toBe(true);
    expect(finds("writ(e|es|ing|ten)", "written")).toBe(true);
    expect(finds("writ(e|es|ing|ten)", "writs")).toBe(false);
    expect(finds("(abc|123)x", "123x")).toBe(true);
    expect(finds("(abc|123)x", "abx")).toBe(false);
  });

  it("lets a quantifier repeat a whole group", () => {
    expect(finds("(ab)+", "ababab")).toBe(true);
    expect(finds("(ab)+c", "abc")).toBe(true);
    expect(finds("(ab)+c", "ac")).toBe(false);
    expect(finds("(a|b){1,2}c", "bac")).toBe(true);
    expect(finds("(a|b){1,2}c", "c")).toBe(false);
  });

  it("rejects a group it cannot close", () => {
    expect(error("(ab")).toMatch(/not closed/);
    expect(error("a)")).toMatch(/matching/);
  });
});

describe("anchors", () => {
  it("takes ^ and $ as the start and the end of the text", () => {
    expect(finds("^kit", "kitab")).toBe(true);
    expect(finds("^tab", "kitab")).toBe(false);
    expect(finds("tab$", "kitab")).toBe(true);
    expect(finds("^kitab$", "kitab")).toBe(true);
    expect(finds("^kitab$", "kitabs")).toBe(false);
    expect(finds("^a$", "a")).toBe(true);
    expect(finds("^$^", "")).toBe(true);
  });
});

describe("staying quick", () => {
  it("runs in one pass on patterns that would trap a backtracking engine", () => {
    const long = "a".repeat(5000) + "b";
    expect(finds("(a|a)*b", long)).toBe(true);
    expect(finds("(a*)*b", long)).toBe(true);
    expect(finds("(a|a)*c", long)).toBe(false);
  });
});

describe("the review pass", () => {
  it("takes | inside a class as a literal member", () => {
    expect(finds("[a|b]c", "|c")).toBe(true);
    expect(finds("[a|b]c", "bc")).toBe(true);
    expect(finds("[a|b]c", "ac")).toBe(true);
    expect(finds("[a|b]c", "c")).toBe(false);
  });
});

describe("a quantifier over a group that holds a choice or a quantifier", () => {
  it("repeats the whole group, not only its first piece", () => {
    expect(finds("^(a|b)*c$", "abbac")).toBe(true);
    expect(finds("^(a|b)*c$", "abxc")).toBe(false);
    expect(finds("^(a?b)*$", "abbab")).toBe(true);
    expect(finds("^(a?b)*$", "abba")).toBe(false);
    expect(finds("^(ab|cd)+$", "abcdab")).toBe(true);
    expect(finds("^(ab|cd){1,3}$", "abcdab")).toBe(true);
    expect(finds("^(ab|cd){1,3}$", "abcdabcd")).toBe(false);
    expect(finds("^(a|b){2,}$", "abab")).toBe(true);
    expect(finds("^(a|b){2,}$", "a")).toBe(false);
    expect(finds("^x(a|b)?y$", "xy")).toBe(true);
    expect(finds("^x(a|b)?y$", "xby")).toBe(true);
    expect(finds("^x(a|b)?y$", "xaby")).toBe(false);
    expect(finds("^((a|b)c)*$", "acbcac")).toBe(true);
  });
});

describe("a pattern that would grow too big", () => {
  it("is refused with a message, however the size is reached", () => {
    expect(error("(a{100,100}){100,100}")).toBeNull();
    expect(error("((a{100,100}){100,100}){100,100}")).toMatch(/too big/);
    expect(error("(((a{100,100}){100,100}){100,100}){100,100}")).toMatch(/too big/);
    expect(error("(a{100,100}){100,100}|(a{100,100}){100,100}")).toMatch(/too big/);
  });
});

describe("characters outside the basic plane", () => {
  it("takes one such character as one literal, one class member and one any character", () => {
    expect(finds("a\u{1F600}b", "a\u{1F600}b")).toBe(true);
    expect(finds("[\u{1F600}]", "\u{1F600}")).toBe(true);
    expect(finds("^a.b$", "a\u{1F600}b")).toBe(true);
    expect(finds("^\u{1F600}{1,2}$", "\u{1F600}\u{1F600}")).toBe(true);
  });
});

describe("a compiled pattern used on many texts", () => {
  it("answers each text on its own", () => {
    const compiled = compilePattern("^a(b|c)*d$");
    if (typeof compiled === "string") throw new Error(compiled);
    const answers = ["abcd", "ad", "abx", "abcd", "", "abbbd", "ad"].map((text) => compiled.matches(text));
    expect(answers).toEqual([true, true, false, true, false, true, true]);
  });
});
