import type { ErrorCode, Limits } from "../src/syntax/types";

export interface MatchRow {
  pattern: string;
  text: string;
  matches: boolean;
}

export interface RefusedRow {
  pattern: string;
  code: ErrorCode;
  position?: number;
  detail?: string;
  limits?: Partial<Limits>;
}

export interface Area {
  name: string;
  matches: MatchRow[];
  refused: RefusedRow[];
}

const rows = (matches: boolean) => (pattern: string, ...texts: string[]): MatchRow[] => texts.map((text) => ({ pattern, text, matches }));
export const yes = rows(true);
export const no = rows(false);

export const LITERALS_AND_STRUCTURE: Area = {
  name: "literals, alternation and groups",
  matches: [
    ...yes("kitab", "kitab", "the book kitab"),
    ...yes("kit", "kitab"),
    ...yes("tab", "kitab"),
    ...no("kitab", "kitbah", ""),
    ...yes("كتاب", "هذا كتاب"),
    ...no("كتاب", "كتب"),
    ...yes("a\u{1F600}b", "a\u{1F600}b"),
    ...no("\u{1F600}", "x", ""),
    ...yes("", "", "abc"),
    ...yes("kitab|katab", "katab", "kitab"),
    ...no("kitab|katab", "kutab"),
    ...yes("writ(e|es|ing|ten)", "written", "writing"),
    ...no("writ(e|es|ing|ten)", "writs"),
    ...yes("(abc|123)x", "123x", "abcx"),
    ...no("(abc|123)x", "abx", "123"),
    ...yes("a|", "b", ""),
    ...yes("|a", "", "a"),
    ...yes("(|a)b", "b", "ab"),
    ...yes("x(a|)y", "xy", "xay"),
    ...no("x(a|)y", "xaay"),
    ...yes("()", "", "abc"),
    ...yes("a()b", "ab"),
    ...no("a()b", "a b"),
    ...yes("a}", "a}"),
    ...yes("}", "}"),
    ...yes("a]", "a]"),
    ...yes("]", "]"),
    ...yes("a|b|c", "c"),
    ...no("a|b|c", "d"),
    ...yes("((a))", "a"),
    ...yes("(a)(b)(c)", "abc"),
    ...no("(a)(b)(c)", "acb"),
  ],
  refused: [
    { pattern: "(ab", code: "missing_paren", position: 0 },
    { pattern: "(", code: "missing_paren", position: 0 },
    { pattern: "((a)", code: "missing_paren", position: 0 },
    { pattern: "a(b(c)", code: "missing_paren", position: 1 },
    { pattern: "a)", code: "unexpected_paren", position: 1 },
    { pattern: ")", code: "unexpected_paren", position: 0 },
    { pattern: "(a))", code: "unexpected_paren", position: 3 },
    { pattern: "a".repeat(201), code: "too_long" },
    { pattern: "(((a)))", code: "nesting_depth", position: 2, limits: { maxDepth: 2 } },
  ],
};

export const AREAS: Area[] = [LITERALS_AND_STRUCTURE];
