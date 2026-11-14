import { describe, expect, it } from "vitest";
import { compilePattern } from "../src";
import { parse } from "../src/syntax/parse";
import { AREAS } from "./contract";
import { referenceMatcher } from "./support/reference";

type Answer = boolean | "error";

const today = (pattern: string, text: string): Answer => {
  const compiled = compilePattern(pattern);
  return typeof compiled === "string" ? "error" : compiled.matches(text);
};

const contract = (pattern: string, text: string): Answer => {
  const parsed = parse(pattern);
  return "error" in parsed ? "error" : referenceMatcher(parsed.tree)(text);
};

const FAMILIES: { name: string; covers: (pattern: string, text: string) => boolean }[] = [
  { name: "an escape that was the plain letter or digit", covers: (pattern) => /\\(?![dDwWsS])[A-Za-z0-9]/.test(pattern) },
  { name: "a backslash before a non-ASCII character, which was that character", covers: (pattern) => /\\[^\u0000-\u007f]/u.test(pattern) },
  { name: "a brace that was text or a count", covers: (pattern) => pattern.includes("{") },
  { name: "a POSIX class that was a class of odd characters", covers: (pattern) => pattern.includes("[:") },
  { name: "the dot on a newline", covers: (pattern, text) => pattern.includes(".") && text.includes("\n") },
];

describe("the current engine against the contract", () => {
  it("answers every contract row as the contract does, except where a declared change applies", () => {
    const undeclared: string[] = [];
    for (const area of AREAS) {
      for (const row of area.matches) {
        if (today(row.pattern, row.text) === row.matches) continue;
        if (!FAMILIES.some((family) => family.covers(row.pattern, row.text))) {
          undeclared.push(`${JSON.stringify(row.pattern)} on ${JSON.stringify(row.text)}: today ${today(row.pattern, row.text)}, contract ${row.matches}`);
        }
      }
      for (const row of area.refused) {
        if (row.limits || today(row.pattern, "") === "error") continue;
        if (!FAMILIES.some((family) => family.covers(row.pattern, ""))) undeclared.push(`${JSON.stringify(row.pattern)}: accepted today, refused by the contract`);
      }
    }
    expect(undeclared).toEqual([]);
  });
});

const BREAKING: { change: string; pattern: string; text: string; before: Answer; after: Answer }[] = [
  { change: "\\n is a newline", pattern: "a\\nb", text: "anb", before: true, after: false },
  { change: "\\n is a newline", pattern: "a\\nb", text: "a\nb", before: false, after: true },
  { change: "\\t is a tab", pattern: "\\t", text: "t", before: true, after: false },
  { change: "\\r is a carriage return", pattern: "\\r", text: "\r", before: false, after: true },
  { change: "\\f is a form feed", pattern: "\\f", text: "f", before: true, after: false },
  { change: "\\v is a vertical tab", pattern: "\\v", text: "\v", before: false, after: true },
  { change: "\\b is a word boundary", pattern: "\\bkit", text: "bkit", before: true, after: false },
  { change: "\\b is a word boundary", pattern: "\\bkit", text: "kit", before: false, after: true },
  { change: "\\B is not a word boundary", pattern: "\\Bkit", text: "xkit", before: false, after: true },
  { change: "\\B is not a word boundary", pattern: "\\Bkit", text: "Bkit", before: true, after: true },
  { change: "\\xHH is one code point", pattern: "\\x41", text: "x41", before: true, after: false },
  { change: "\\xHH is one code point", pattern: "\\x41", text: "A", before: false, after: true },
  { change: "\\uHHHH is one code point", pattern: "\\u0041", text: "u0041", before: true, after: false },
  { change: "\\uHHHH is one code point", pattern: "\\u0041", text: "A", before: false, after: true },
  { change: "an unknown letter escape is an error", pattern: "a\\qb", text: "aqb", before: true, after: "error" },
  { change: "an unknown digit escape is an error", pattern: "a\\1b", text: "a1b", before: true, after: "error" },
  { change: "{n} is a count", pattern: "a{2}", text: "a{2}", before: true, after: false },
  { change: "{n} is a count", pattern: "a{2}", text: "aa", before: false, after: true },
  { change: "{,n} is an error", pattern: "^a{,3}$", text: "aa", before: true, after: "error" },
  { change: "an unclosed brace is an error", pattern: "a{2", text: "a{2", before: true, after: "error" },
  { change: "a lone brace is an error", pattern: "{", text: "{", before: true, after: "error" },
  { change: "the dot does not match a newline", pattern: "a.b", text: "a\nb", before: true, after: false },
  { change: "a POSIX class is refused", pattern: "[[:alpha:]]", text: "a]", before: true, after: "error" },
];

describe("the changes the plan lists, on both sides", () => {
  it.each(BREAKING)("$change: $pattern on $text", ({ pattern, text, before, after }) => {
    expect(today(pattern, text)).toBe(before);
    expect(contract(pattern, text)).toBe(after);
  });
});
