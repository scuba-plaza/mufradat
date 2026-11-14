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

export const DOT_AND_CLASSES: Area = {
  name: "the dot and character classes",
  matches: [
    ...yes(".", "a", "\u00e9", "\u0643", "\u{1F600}", " ", "\r", "\u2028"),
    ...no(".", "", "\n"),
    ...yes("k.b", "ktb"),
    ...no("k.b", "kitb", "kb"),
    ...yes("\u0643.\u0628", "\u0643\u062a\u0628"),
    ...yes("a.b", "a\u{1F600}b"),
    ...no("a.b", "a\nb"),
    ...yes("[abc]", "b"),
    ...no("[abc]", "d", ""),
    ...yes("lesson_[1-3]", "lesson_2"),
    ...no("lesson_[1-3]", "lesson_4"),
    ...yes("[a-d2-6]x", "5x"),
    ...no("[a-d2-6]x", "ex"),
    ...yes("[\u0627-\u064a]", "\u0643\u062a\u0627\u0628"),
    ...no("[\u0627-\u0628]", "\u0643"),
    ...yes("[^a-z]_", "1_"),
    ...no("[^a-z]_", "b_"),
    ...yes("[a^]b", "^b"),
    ...no("[^abc]", "abc", ""),
    ...yes("[^abc]", "abcd", "\n", "\u{1F600}"),
    ...yes("[a-]", "-", "a"),
    ...no("[a-]", "b"),
    ...yes("[-a]", "-", "a"),
    ...yes("[a-c-e]", "-", "e", "b"),
    ...no("[a-c-e]", "d"),
    ...yes("[--a]", "5", "a", "-"),
    ...no("[--a]", "b"),
    ...yes("[a-a]", "a"),
    ...no("[a-a]", "b"),
    ...yes("[a|b]c", "|c", "bc", "ac"),
    ...no("[a|b]c", "c"),
    ...yes("[[]", "["),
    ...yes("[a[b]", "[", "a", "b"),
    ...yes("[.*+?(){}$]", ".", "*", "+", "?", "(", ")", "{", "}", "$"),
    ...no("[.*+?(){}$]", "a"),
    ...yes("[\u{1F600}-\u{1F64F}]", "\u{1F610}"),
    ...no("[\u{1F600}-\u{1F64F}]", "a"),
    ...yes("[\u{1F600}]", "\u{1F600}"),
    ...yes("[a]]", "a]"),
    ...no("[a]]", "a"),
  ],
  refused: [
    { pattern: "[a", code: "missing_bracket", position: 0 },
    { pattern: "[", code: "missing_bracket", position: 0 },
    { pattern: "a[b", code: "missing_bracket", position: 1 },
    { pattern: "[^", code: "missing_bracket", position: 0 },
    { pattern: "[a-", code: "missing_bracket", position: 0 },
    { pattern: "[a-c", code: "missing_bracket", position: 0 },
    { pattern: "[]", code: "empty_class", position: 0 },
    { pattern: "[^]", code: "empty_class", position: 0 },
    { pattern: "[]a]", code: "empty_class", position: 0 },
    { pattern: "a[]", code: "empty_class", position: 1 },
    { pattern: "[z-a]", code: "reversed_char_range", position: 1 },
    { pattern: "x[9-0]", code: "reversed_char_range", position: 2 },
    { pattern: "[^b-a]", code: "reversed_char_range", position: 2 },
  ],
};

export const AREAS: Area[] = [LITERALS_AND_STRUCTURE, DOT_AND_CLASSES];
