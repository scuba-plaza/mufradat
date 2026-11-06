import { SORT_KEYS } from "./fields";
import { closeness } from "./fuzzy";
import type { Query } from "./parse";
import type { WordRow } from "../types";

function sortedBy(rows: WordRow[], sort: NonNullable<Query["sort"]>): WordRow[] {
  const key = SORT_KEYS[sort.key];
  const direction = sort.descending ? -1 : 1;
  return [...rows].sort((a, b) => {
    const [x, y] = [key(a), key(b)];
    if (x === null || y === null) return x === y ? 0 : x === null ? 1 : -1;
    return x === y ? 0 : x < y ? -direction : direction;
  });
}

function textDistance(row: WordRow, texts: Query["texts"]): number {
  let total = 0;
  for (const { term, phrase } of texts) {
    const near = Math.min(closeness(term, row.lemma, phrase), closeness(term, row.english, phrase));
    if (near === Infinity) return Infinity;
    total += near;
  }
  return total;
}

export function runQuery(rows: WordRow[], query: Query, now: Date): WordRow[] {
  const kept = rows.filter((row) => query.tests.every((test) => test(row, now)));
  if (query.texts.length === 0) return query.sort ? sortedBy(kept, query.sort) : kept;
  const matches = kept.map((row) => ({ row, near: textDistance(row, query.texts) })).filter((m) => m.near !== Infinity);
  if (query.sort) return sortedBy(matches.map((m) => m.row), query.sort);
  return matches.sort((a, b) => a.near - b.near).map((m) => m.row);
}
