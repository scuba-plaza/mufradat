import { cardForRank, lookup, rankedWords, wiktionaryWords } from "./lexicon";
import { missingKinds, wordStatus } from "./missing";
import { parseQuery } from "./query/parse";
import { runQuery } from "./query/run";
import type { Store } from "./store";
import { MISSING_KINDS, POS_VALUES, type Note, type POS, type WordRow } from "./types";

const fromCard = (n: Note): WordRow => ({
  id: n.id,
  rank: n.rank > 0 ? n.rank : null,
  lemma: n.lemma,
  english: n.english,
  pos: [n.pos],
  gender: n.gender,
  root: n.root,
  verbForm: n.verbForm,
  status: wordStatus(n),
  missing: missingKinds(n),
  error: n.exampleError !== null || Object.values(n.audio).some((clip) => clip.error !== null),
  exampleBy: n.exampleSource,
  edited: n.updatedAt,
});

const notStarted = (row: Pick<WordRow, "rank" | "lemma" | "english" | "pos" | "gender" | "root" | "verbForm">): WordRow => ({
  ...row,
  id: null,
  status: "todo",
  missing: [...MISSING_KINDS],
  error: false,
  exampleBy: "",
  edited: null,
});

function listRows(notes: Note[]): WordRow[] {
  const byRank = new Map(notes.filter((n) => n.rank > 0).map((n) => [n.rank, n]));
  const listed = rankedWords().map((w) => {
    const card = byRank.get(w.rank);
    return card ? fromCard(card) : notStarted(w);
  });
  const added = notes.filter((n) => n.rank === 0).sort((a, b) => a.createdAt.localeCompare(b.createdAt)).map(fromCard);
  return [...listed, ...added];
}

function entryRows(notes: Note[]): WordRow[] {
  const made = new Set(notes.map((n) => `${n.lemma}|${n.pos}`));
  return wiktionaryWords()
    .filter((e) => e.rank === 0 && !made.has(`${e.lemma}|${e.pos}`))
    .map((e) => notStarted({ ...e, rank: null, pos: [e.pos] }));
}

/** All 5000 frequency words in rank order, each showing its card when it has one, then the cards made outside the list. */
export const wordRows = (store: Store): WordRow[] => listRows(store.allNotes());

const working = (r: WordRow) => r.status === "progress" || r.status === "ready";
const group = (r: WordRow) => (working(r) ? 0 : r.status === "todo" ? 1 : 2);

function byRank(a: WordRow, b: WordRow): number {
  if (a.rank === b.rank) return 0;
  if (a.rank === null) return 1;
  if (b.rank === null) return -1;
  return a.rank - b.rank;
}

function latestFirst(a: WordRow, b: WordRow): number {
  const [x, y] = [a.edited ?? "", b.edited ?? ""];
  return x === y ? 0 : x < y ? 1 : -1;
}

/** Cards being worked on first, latest edit on top; then words not started by rank; then approved ones by rank. */
export function defaultOrder(rows: WordRow[]): WordRow[] {
  return [...rows].sort(
    (a, b) => group(a) - group(b) || (group(a) === 0 ? latestFirst(a, b) || byRank(a, b) : byRank(a, b) || latestFirst(a, b)),
  );
}

/** What Continue opens: the first row of the default order that is not approved. */
export function continueFrom(rows: WordRow[]): WordRow | null {
  return defaultOrder(rows).find((r) => r.status !== "approved") ?? null;
}

export const continueTarget = (store: Store): WordRow | null => continueFrom(wordRows(store));

export interface View {
  rows: WordRow[];
  error: string | null;
}

export function view(store: Store, text: string, now: Date = new Date()): View {
  const parsed = parseQuery(text);
  if ("error" in parsed) return { rows: [], error: parsed.error };
  const notes = store.allNotes();
  const rows = listRows(notes);
  if (parsed.query.texts.length > 0) rows.push(...entryRows(notes));
  return { rows: runQuery(defaultOrder(rows), parsed.query, now), error: null };
}

/** The id of the card for a ranked word or a Wiktionary entry, creating the bare card if there is none. */
export function openCard(store: Store, target: { rank: number } | { lemma: string; pos: string }): string | null {
  const card =
    "rank" in target
      ? cardForRank(target.rank)
      : POS_VALUES.includes(target.pos as POS)
        ? lookup(target.lemma).find((c) => c.lemma === target.lemma && c.pos === target.pos)
        : undefined;
  if (!card) return null;
  return store.findNote(card.lemma, card.pos) ?? store.createNote(card).id;
}

/** Where "next" leads: the card Continue opens, created bare on first use, or the words page when none is left. */
export function nextCardPath(store: Store): string {
  const target = continueTarget(store);
  const id = target?.id ?? (target?.rank ? openCard(store, { rank: target.rank }) : null);
  return id ? `/cards/${id}` : "/";
}

export function rowHref(row: WordRow): string {
  if (row.id) return `/cards/${row.id}`;
  if (row.rank) return `/open?rank=${row.rank}`;
  return `/open?${new URLSearchParams({ lemma: row.lemma, pos: row.pos[0] })}`;
}
