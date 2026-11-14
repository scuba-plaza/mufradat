import { fold } from "./fuzzy";
import { compilePattern, type Pattern } from "./pattern";
import { FIELDS, SORT_KEYS, listed, type Field, type SortKey, type Test } from "./fields";

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

const SORT_NAMES = Object.keys(SORT_KEYS);

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
      continue;
    }
    if (key === "sort") {
      if (minus) return { error: "sort: takes its minus inside the value, as in sort:-edited." };
      if (query.sort) return { error: "Only one sort: is allowed." };
      const descending = value.startsWith("-");
      const sortKey = (descending ? value.slice(1) : value).toLowerCase();
      if (!SORT_NAMES.includes(sortKey)) {
        return { error: `sort: expects ${listed(SORT_NAMES)}, optionally with a minus in front, not "${value.replace(/^-/, "")}".` };
      }
      query.sort = { key: sortKey as SortKey, descending };
      continue;
    }
    const def = FIELDS[key];
    if (!def) return { error: `Unknown field "${name}". Fields: ${[...Object.keys(FIELDS), "sort"].join(", ")}.` };
    const parts = isPattern(value) ? [value] : value.split(",");
    if (parts.some((p) => p.trim() === "")) return { error: `${key}: needs a value.` };
    if (parts.length > MAX_VALUES) return { error: `${key}: at most ${MAX_VALUES} values.` };
    const tests: Test[] = [];
    for (const part of parts) {
      const text = part.trim();
      const pattern = isPattern(text);
      const test = pattern ? patternTest(key, def, text.slice(RE.length)) : def.parse(text);
      if (typeof test === "string") return { error: pattern ? test : `${key}: ${test}` };
      tests.push(test);
    }
    const anyOf: Test = (row, now) => tests.some((t) => t(row, now));
    query.tests.push(minus ? (row, now) => !anyOf(row, now) : anyOf);
    if (key === "status" && !minus && parts.some((p) => p.trim().toLowerCase() === "suspended")) query.suspended = true;
  }
  return { query };
}

const RE = "re:";

const isPattern = (text: string) => text.slice(0, RE.length).toLowerCase() === RE;

const freeText = (pattern: Pattern): Test => (row) => pattern.matches(fold(row.lemma)) || pattern.matches(fold(row.english));

const foldPattern = (source: string): string =>
  source.replace(/\\([^])|[^\\]+/gu, (piece: string, escaped?: string) => {
    if (escaped === undefined) return fold(piece);
    if (/[DWS]/.test(escaped)) return piece;
    const folded = fold(escaped);
    return folded === "" ? "" : `\\${folded}`;
  });

function patternTest(key: string, def: Pick<Field, "pattern">, body: string): Test | string {
  const label = key === "re" ? RE : `${key}:${RE}`;
  if (body === "") return `${label} needs a pattern.`;
  if (!def.pattern) return `${key}: takes fixed values, not a pattern.`;
  const found = compilePattern(foldPattern(body));
  return typeof found === "string" ? `${label} ${found}` : def.pattern(found);
}

export function withSort(text: string, sort: string | null): string {
  const terms = tokenize(text).tokens.map((t) => t.raw).filter((raw) => !/^sort:/i.test(raw));
  return (sort ? [...terms, `sort:${sort}`] : terms).join(" ");
}

export function nextSort(sort: Query["sort"], key: SortKey): string | null {
  if (sort?.key !== key) return key;
  return sort.descending ? null : `-${key}`;
}
