import { cardForRank, lookup, rankedWords, rankingSize, type Word } from "./lexicon";
import type { Store } from "./store";
import { POS_VALUES, type CardStatus, type POS } from "./types";

/** A word of the list. `status` is that of its card, and absent while the word has no card. */
interface ListedWord extends Word {
  status?: CardStatus;
}

/** All ranked words in order. A word that has a card shows the card's text and status. */
export function frequencyList(store: Store): ListedWord[] {
  const cards = store.rankedCards();
  return rankedWords().map((w) => ({ rank: w.rank, ...(cards.get(w.rank) ?? w) }));
}

/** The first word without a card that was not skipped, if any. */
export function nextWord(store: Store): Word | null {
  const rank = store.nextRank(rankingSize());
  return rank === null ? null : (frequencyList(store).find((w) => w.rank === rank) ?? null);
}

/** The id of the card for a ranked word or a Wiktionary entry, creating the bare card if there is none. */
export function openCard(store: Store, target: { rank: number } | { lemma: string; pos: string }): string | null {
  const card =
    "rank" in target
      ? cardForRank(target.rank)
      : POS_VALUES.includes(target.pos as POS)
        ? lookup(target.lemma).find((c) => c.pos === target.pos)
        : undefined;
  if (!card) return null;
  return store.findNote(card.lemma, card.pos) ?? store.createNote(card).id;
}

/** Where "next" leads: the card of the next word that is not approved, created bare on first use, or the words page when none is left. */
export function nextCardPath(store: Store): string {
  const next = nextWord(store);
  const id = next ? openCard(store, { rank: next.rank }) : null;
  return id ? `/cards/${id}` : "/";
}
