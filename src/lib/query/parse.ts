import { fold } from "./fuzzy";
import { compilePattern, type Pattern } from "./pattern";
import { FIELDS, SORT_KEYS, listed, type SortKey, type Test } from "./fields";

export interface Query {
  texts: { term: string; phrase: boolean }[];
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
  const query: Query = { texts: [], tests: [], sort: null, suspended: false };
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
      continue;
    }
    const [, minus, name, value] = field;
    const key = name.toLowerCase();
    if (key === "re") {
      if (minus) return { error: "A minus goes in front of a field term, such as -pos:verb." };
      if (value === "") return { error: "re: needs a pattern." };
      const found = compilePattern(value);
      if (typeof found === "string") return { error: `re: ${found}` };
      query.tests.push(freeText(found));
      continue;
    }
    if (key === "sort") {
      if (minus) return { error: "sort: takes its minus inside the value, as in sort:-edited." };
      if (value.startsWith("re:")) return { error: "sort: takes rank, edited, arabic or english, not a pattern." };
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
    const tests: Test[] = [];
    let parts: string[] | null = null;
    if (value.startsWith("re:")) {
      const test = patternValue(key, def, value);
      if (typeof test === "string") return { error: test };
      tests.push(test);
    } else {
      parts = value.split(",");
      if (parts.some((p) => p.trim() === "")) return { error: `${key}: needs a value.` };
      if (parts.length > MAX_VALUES) return { error: `${key}: at most ${MAX_VALUES} values.` };
      for (const part of parts) {
        const test = part.startsWith("re:") ? patternValue(key, def, part) : def.parse(part.trim());
        if (typeof test === "string") return { error: `${key}: ${test}` };
        tests.push(test);
      }
    }
    const anyOf: Test = (row, now) => tests.some((t) => t(row, now));
    query.tests.push(minus ? (row, now) => !anyOf(row, now) : anyOf);
    if (key === "status" && !minus && parts !== null && parts.some((p) => p.trim().toLowerCase() === "suspended")) query.suspended = true;
  }
  return { query };
}

const freeText = (pattern: Pattern): Test => (row) => pattern.matches(fold(row.lemma)) || pattern.matches(fold(row.english));

function patternValue(key: string, def: { pattern?: (p: Pattern) => Test }, value: string): Test | string {
  const body = value.slice(3);
  if (body === "") return `${key}:re: needs a pattern.`;
  if (!def.pattern) return `${key}: takes fixed values, not a pattern.`;
  const found = compilePattern(body);
  if (typeof found === "string") return `${key}:re: ${found}`;
  return def.pattern(found);
}

export function withSort(text: string, sort: string | null): string {
  const terms = tokenize(text).tokens.map((t) => t.raw).filter((raw) => !/^sort:/i.test(raw));
  return (sort ? [...terms, `sort:${sort}`] : terms).join(" ");
}

export function nextSort(sort: Query["sort"], key: SortKey): string | null {
  if (sort?.key !== key) return key;
  return sort.descending ? null : `-${key}`;
}
