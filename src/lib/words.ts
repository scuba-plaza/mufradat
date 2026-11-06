import { cardForRank, lookup, rankedWords } from "./lexicon";
import { wordStatus } from "./missing";
import type { Store } from "./store";
import { POS_VALUES, type Note, type POS, type WordStatus } from "./types";

/** One word of the list: a card, or a frequency word that has none yet. */
export interface WordRow {
  id: string | null;
  rank: number | null;
  lemma: string;
  english: string;
  status: WordStatus;
  edited: string | null;
}

const fromCard = (n: Note): WordRow => ({
  id: n.id,
  rank: n.rank > 0 ? n.rank : null,
  lemma: n.lemma,
  english: n.english,
  status: wordStatus(n),
  edited: n.updatedAt,
});

/** All 5000 frequency words in rank order, each showing its card when it has one, then the cards made outside the list. */
export function wordRows(store: Store): WordRow[] {
  const notes = store.allNotes();
  const byRank = new Map(notes.filter((n) => n.rank > 0).map((n) => [n.rank, n]));
  const listed = rankedWords().map((w): WordRow => {
    const card = byRank.get(w.rank);
    return card ? fromCard(card) : { id: null, rank: w.rank, lemma: w.lemma, english: w.english, status: "todo", edited: null };
  });
  const added = notes.filter((n) => n.rank === 0).sort((a, b) => a.createdAt.localeCompare(b.createdAt)).map(fromCard);
  return [...listed, ...added];
}

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
