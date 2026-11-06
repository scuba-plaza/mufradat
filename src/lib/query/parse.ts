import { FIELDS, SORT_KEYS, listed, type SortKey, type Test } from "./fields";

export interface Query {
  texts: { term: string; phrase: boolean }[];
  tests: Test[];
  sort: { key: SortKey; descending: boolean } | null;
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
  const query: Query = { texts: [], tests: [], sort: null };
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
    const parts = value.split(",");
    if (parts.some((p) => p.trim() === "")) return { error: `${key}: needs a value.` };
    if (parts.length > MAX_VALUES) return { error: `${key}: at most ${MAX_VALUES} values.` };
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
    const tests: Test[] = [];
    for (const part of parts) {
      const test = def.parse(part.trim());
      if (typeof test === "string") return { error: `${key}: ${test}` };
      tests.push(test);
    }
    const anyOf: Test = (row, now) => tests.some((t) => t(row, now));
    query.tests.push(minus ? (row, now) => !anyOf(row, now) : anyOf);
  }
  return { query };
}

export function withSort(text: string, sort: string | null): string {
  const terms = tokenize(text).tokens.map((t) => t.raw).filter((raw) => !/^sort:/i.test(raw));
  return (sort ? [...terms, `sort:${sort}`] : terms).join(" ");
}

export function nextSort(sort: Query["sort"], key: SortKey): string | null {
  if (sort?.key !== key) return key;
  return sort.descending ? null : `-${key}`;
}
