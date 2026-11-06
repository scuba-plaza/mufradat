import { FIELDS, SORT_KEYS, listed, type SortKey, type Test } from "./fields";

export interface Query {
  texts: { term: string; phrase: boolean }[];
  tests: Test[];
  sort: { key: SortKey; descending: boolean } | null;
}

export type Parsed = { query: Query } | { error: string };

interface Token {
  raw: string;
  value: string;
  quoted: boolean;
}

function tokenize(text: string): Token[] {
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
  flush();
  return tokens;
}

const SORT_NAMES = Object.keys(SORT_KEYS);

export function parseQuery(text: string): Parsed {
  const query: Query = { texts: [], tests: [], sort: null };
  for (const token of tokenize(text)) {
    const field = token.quoted ? null : /^(-?)([A-Za-z]+):(.*)$/s.exec(token.value);
    if (!field) {
      if (!token.quoted && token.value.startsWith("-")) return { error: "A minus goes in front of a field term, such as -pos:verb." };
      query.texts.push({ term: token.value, phrase: token.quoted });
      continue;
    }
    const [, minus, name, value] = field;
    const key = name.toLowerCase();
    const parts = value.split(",");
    if (parts.some((p) => p.trim() === "")) return { error: `${key}: needs a value.` };
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
    const any: Test = (row, now) => tests.some((t) => t(row, now));
    query.tests.push(minus ? (row, now) => !any(row, now) : any);
  }
  return { query };
}

export function withSort(text: string, sort: string | null): string {
  const terms = tokenize(text).map((t) => t.raw).filter((raw) => !/^sort:/i.test(raw));
  return (sort ? [...terms, `sort:${sort}`] : terms).join(" ");
}

export function nextSort(sort: Query["sort"], key: SortKey): string | null {
  if (sort?.key !== key) return key;
  return sort.descending ? null : `-${key}`;
}
