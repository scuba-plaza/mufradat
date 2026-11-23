import { compile, type Matcher } from "@mufradat/regex";
import { describeError } from "./errors";
import { FIELDS, SORT_KEYS, listed, type Field, type SortKey, type Test } from "./fields";
import { fold } from "./fuzzy";

export interface Query {
  texts: { term: string; phrase: boolean }[];
  searchesText: boolean;
  tests: Test[];
  sort: { key: SortKey; descending: boolean } | null;
  /** True when the query asks for suspended words with a positive status:suspended term. */
  suspended: boolean;
}

export type Parsed = { query: Query } | { error: string };

const MAX_TERMS = 12;
const MAX_VALUES = 12;
const RE = "re:";
const SORT_NAMES = Object.keys(SORT_KEYS);

interface Token {
  raw: string;
  value: string;
  quoted: boolean;
}

function tokenize(text: string): { tokens: Token[]; unclosed: boolean } {
  const tokens: Token[] = [];
  let [raw, value, quoted, open] = ["", "", false, false];
  const flush = () => {
    if (raw !== "" && !(quoted && value === "")) tokens.push({ raw, value, quoted });
    [raw, value, quoted, open] = ["", "", false, false];
  };
  for (const char of text) {
    if (char === '"') {
      if (raw === "") quoted = true;
      open = !open;
      raw += char;
    } else if (/\s/.test(char) && !open) flush();
    else {
      raw += char;
      value += char;
    }
  }
  const unclosed = open;
  flush();
  return { tokens, unclosed };
}

export function parseQuery(text: string): Parsed {
  const query: Query = { texts: [], searchesText: false, tests: [], sort: null, suspended: false };
  const { tokens, unclosed } = tokenize(text);
  if (unclosed) return { error: "A quote is not closed." };
  if (tokens.length > MAX_TERMS) return { error: `At most ${MAX_TERMS} terms.` };
  for (const token of tokens) {
    const field = token.quoted ? null : /^(-?)([A-Za-z]+):(.*)$/s.exec(token.value);
    if (!field) {
      if (!token.quoted && token.value.startsWith("-")) {
        return { error: `A minus goes in front of a field term, such as -pos:verb. To find text that starts with one, put it in quotes: "${token.value}".` };
      }
      query.texts.push({ term: token.value, phrase: token.quoted || /\s/.test(token.value) });
      query.searchesText = true;
      continue;
    }
    const [, minus, name, value] = field;
    const key = name.toLowerCase();
    if (key === "re") {
      if (minus) return { error: "re: cannot be negated; put the minus before a field, as in -tag:re:lesson_.*." };
      const test = patternTest(key, { pattern: freeText }, value);
      if (typeof test === "string") return { error: test };
      query.tests.push(test);
      query.searchesText = true;
    } else if (key === "sort") {
      if (minus) return { error: "sort: takes its minus inside the value, as in sort:-edited." };
      if (query.sort) return { error: "Only one sort: is allowed." };
      const sort = parseSort(value);
      if (typeof sort === "string") return { error: sort };
      query.sort = sort;
    } else {
      const def = FIELDS[key];
      if (!def) return { error: `Unknown field "${name}". Fields: ${[...Object.keys(FIELDS), "sort"].join(", ")}.` };
      const tests = fieldTests(key, def, value);
      if (typeof tests === "string") return { error: tests };
      const anyOf: Test = (row, now) => tests.some((t) => t(row, now));
      query.tests.push(minus ? (row, now) => !anyOf(row, now) : anyOf);
      if (key === "status" && !minus && value.split(",").some((p) => p.trim().toLowerCase() === "suspended")) query.suspended = true;
    }
  }
  return { query };
}

function parseSort(value: string): NonNullable<Query["sort"]> | string {
  const descending = value.startsWith("-");
  const key = (descending ? value.slice(1) : value).toLowerCase();
  if (!SORT_NAMES.includes(key)) {
    return `sort: expects ${listed(SORT_NAMES)}, optionally with a minus in front, not "${value.replace(/^-/, "")}".`;
  }
  return { key: key as SortKey, descending };
}

function fieldTests(key: string, def: Field, value: string): Test[] | string {
  const parts = isPattern(value) ? [value] : value.split(",");
  if (parts.some((p) => p.trim() === "")) return `${key}: needs a value.`;
  if (parts.length > MAX_VALUES) return `${key}: at most ${MAX_VALUES} values.`;
  const tests: Test[] = [];
  for (const part of parts) {
    const text = part.trim();
    const pattern = isPattern(text);
    const test = pattern ? patternTest(key, def, text.slice(RE.length)) : def.parse(text);
    if (typeof test === "string") return pattern ? test : `${key}: ${test}`;
    tests.push(test);
  }
  return tests;
}

const isPattern = (text: string) => text.slice(0, RE.length).toLowerCase() === RE;

const freeText = (pattern: Matcher): Test => (row) => pattern.test(fold(row.lemma)) || pattern.test(fold(row.english));

const MAX_CODE_POINT = 0x10ffff;
const PIECES = /\\(?:x\{[^}]*\}?|x[^]{0,2}|u[^]{0,4}|[pP](?:\{[^}]*\}?|[^])?|[^])|[^\\]+/gu;
const CODE_ESCAPE = /^\\(?:x\{([0-9a-f]+)\}|x([0-9a-f]{2})|u([0-9a-f]{4}))$/iu;

const escapeOf = (char: string): string => `\\x{${char.codePointAt(0)!.toString(16)}}`;

const foldPattern = (source: string): string =>
  source.replace(PIECES, (piece: string) => {
    if (!piece.startsWith("\\")) return fold(piece);
    const digits = CODE_ESCAPE.exec(piece)?.slice(1).find((group) => group !== undefined);
    const code = digits === undefined ? NaN : parseInt(digits, 16);
    return code <= MAX_CODE_POINT ? [...fold(String.fromCodePoint(code))].map(escapeOf).join("") : piece;
  });

function patternTest(key: string, def: Pick<Field, "pattern">, body: string): Test | string {
  const label = key === "re" ? RE : `${key}:${RE}`;
  if (body === "") return `${label} needs a pattern.`;
  if (!def.pattern) return `${key}: takes fixed values, not a pattern.`;
  const found = compile(foldPattern(body));
  return "code" in found ? `${label} ${describeError(found)}` : def.pattern(found);
}

export function withSort(text: string, sort: string | null): string {
  const terms = tokenize(text).tokens.map((t) => t.raw).filter((raw) => !/^sort:/i.test(raw));
  return (sort ? [...terms, `sort:${sort}`] : terms).join(" ");
}

export function nextSort(sort: Query["sort"], key: SortKey): string | null {
  if (sort?.key !== key) return key;
  return sort.descending ? null : `-${key}`;
}
