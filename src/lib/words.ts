import { cardForRank, lookup, rankedWords, wiktionaryWords } from "./lexicon";
import { wordStatus } from "./missing";
import { parseQuery, type Query } from "./query/parse";
import { runQuery } from "./query/run";
import type { Store } from "./store";
import { POS_VALUES, type NewNote, type Note, type POS, type WordRow } from "./types";

const fromCard = (n: Note): WordRow => ({
  id: n.id,
  rank: n.rank > 0 ? n.rank : null,
  lemma: n.lemma,
  english: n.english,
  pos: n.pos,
  gender: n.gender,
  root: n.root,
  verbForm: n.verbForm,
  status: wordStatus(n),
  edited: n.updatedAt,
  tags: n.tags,
});

const notStarted = (row: Pick<WordRow, "rank" | "lemma" | "english" | "pos" | "gender" | "root" | "verbForm">): WordRow => ({
  ...row,
  id: null,
  status: "todo",
  edited: null,
  tags: [],
});

const entryKey = (lemma: string, pos: POS) => `${lemma}|${pos}`;

function listRows(notes: Note[], withEntries: boolean): WordRow[] {
  const byRank = new Map(notes.filter((n) => n.rank > 0).map((n) => [n.rank, n]));
  const listed = rankedWords().map((w) => {
    const card = byRank.get(w.rank);
    return card ? fromCard(card) : notStarted(w);
  });
  const added = notes.filter((n) => n.rank === 0).sort((a, b) => a.createdAt.localeCompare(b.createdAt));
  if (!withEntries) return [...listed, ...added.map(fromCard)];
  const cards = new Map(added.map((n) => [entryKey(n.lemma, n.pos), n]));
  const entries = wiktionaryWords().filter((e) => e.rank === 0);
  const listedEntries = new Set(entries.map((e) => entryKey(e.lemma, e.pos)));
  return [
    ...listed,
    ...entries.map((e) => {
      const card = cards.get(entryKey(e.lemma, e.pos));
      return card ? fromCard(card) : notStarted({ ...e, rank: null });
    }),
    ...added.filter((n) => !listedEntries.has(entryKey(n.lemma, n.pos))).map(fromCard),
  ];
}

const notSuspended = (r: WordRow) => r.status !== "suspended";

export const wordRows = (store: Store): WordRow[] => listRows(store.allNotes(), false).filter(notSuspended);

/** Cards being worked on come first, then words not started, then approved cards. */
function group(r: WordRow): number {
  if (r.status === "progress" || r.status === "ready") return 0;
  if (r.status === "todo") return 1;
  return 2;
}

function byRank(a: WordRow, b: WordRow): number {
  if (a.rank === b.rank) return 0;
  if (a.rank === null) return 1;
  if (b.rank === null) return -1;
  return a.rank - b.rank;
}

function latestFirst(a: WordRow, b: WordRow): number {
  const [x, y] = [a.edited ?? "", b.edited ?? ""];
  if (x === y) return 0;
  return x < y ? 1 : -1;
}

function defaultCompare(a: WordRow, b: WordRow): number {
  const byGroup = group(a) - group(b);
  if (byGroup !== 0) return byGroup;
  if (group(a) === 0) return latestFirst(a, b) || byRank(a, b);
  return byRank(a, b) || latestFirst(a, b);
}

export function defaultOrder(rows: WordRow[]): WordRow[] {
  return [...rows].sort(defaultCompare);
}

export function continueFrom(rows: WordRow[]): WordRow | null {
  return defaultOrder(rows).find((r) => r.status !== "approved") ?? null;
}

export const continueTarget = (store: Store): WordRow | null => continueFrom(wordRows(store));

export const approvedIds = (rows: WordRow[]): string[] =>
  rows.flatMap((r) => (r.status === "approved" && r.id !== null ? [r.id] : []));

interface View {
  rows: WordRow[];
  error: string | null;
  sort: Query["sort"];
}

function evaluate(store: Store, text: string, now: Date): { error: string } | { query: Query; start: WordRow[]; rows: WordRow[] } {
  const parsed = parseQuery(text);
  if ("error" in parsed) return { error: parsed.error };
  const every = listRows(store.allNotes(), parsed.query.searchesText);
  const all = parsed.query.suspended ? every : every.filter(notSuspended);
  const start = text.trim() === "" ? defaultOrder(all) : all;
  return { query: parsed.query, start, rows: runQuery(start, parsed.query, now) };
}

export function view(store: Store, text: string, now: Date = new Date()): View {
  const found = evaluate(store, text, now);
  return "error" in found ? { rows: [], error: found.error, sort: null } : { rows: found.rows, error: null, sort: found.query.sort };
}

function bareCard(target: { rank: number } | { lemma: string; pos: string }): NewNote | null {
  if ("rank" in target) return cardForRank(target.rank);
  if (!POS_VALUES.includes(target.pos as POS)) return null;
  return lookup(target.lemma).find((c) => c.lemma === target.lemma && c.pos === target.pos) ?? null;
}

/** The id of the card for a ranked word or a Wiktionary entry, creating the bare card if there is none. */
export function openCard(store: Store, target: { rank: number } | { lemma: string; pos: string }): string | null {
  const card = bareCard(target);
  if (!card) return null;
  return store.findNote(card.lemma, card.pos) ?? store.createNote(card).id;
}

/** Where "next" leads: the card Continue opens, created bare on first use, or the words page when none is left. */
export function nextCardPath(store: Store): string {
  const target = continueTarget(store);
  const id = target?.id ?? (target?.rank ? openCard(store, { rank: target.rank }) : null);
  return id ? `/cards/${id}` : "/";
}

const WALK_ALL = "-rank:none sort:rank";

export function neighbour(store: Store, id: string, q: string, direction: "previous" | "next", now: Date = new Date()): WordRow | null {
  const walkingAll = q.trim() === "";
  const found = evaluate(store, walkingAll ? WALK_ALL : q, now);
  if ("error" in found) return null;
  const { rows } = found;
  const at = rows.findIndex((r) => r.id === id);
  if (at >= 0) return (direction === "next" ? rows[at + 1] : rows[at - 1]) ?? null;
  if (walkingAll) return null;
  const whole = runQuery(found.start, { ...found.query, tests: [] }, now);
  const from = whole.findIndex((r) => r.id === id);
  if (from < 0) return null;
  const place = new Map(whole.map((r, i) => [r, i]));
  const after = (r: WordRow) => place.get(r)! > from;
  return (direction === "next" ? rows.find(after) : [...rows].reverse().find((r) => !after(r))) ?? null;
}

export function cardPath(id: string, q = ""): string {
  return q === "" ? `/cards/${id}` : `/cards/${id}?${new URLSearchParams({ q })}`;
}

export function rowHref(row: WordRow, q = ""): string {
  if (row.id) return cardPath(row.id, q);
  const params = new URLSearchParams(row.rank ? { rank: String(row.rank) } : { lemma: row.lemma, pos: row.pos });
  if (q !== "") params.set("q", q);
  return `/open?${params}`;
}
