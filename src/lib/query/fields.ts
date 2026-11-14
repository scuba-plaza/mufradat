import { POS_VALUES, type WordRow } from "../types";
import { fold } from "./fuzzy";
import type { Pattern } from "@mufradat/regex";

export type Test = (row: WordRow, now: Date) => boolean;

export interface Field {
  parse(part: string): Test | string;
  /** When set, the field also takes re: followed by a pattern. */
  pattern?: (pattern: Pattern) => Test;
}

export const SORT_KEYS = {
  rank: (row: WordRow) => row.rank,
  edited: (row: WordRow) => row.edited,
  arabic: (row: WordRow) => fold(row.lemma),
  english: (row: WordRow) => row.english.toLowerCase(),
} satisfies Record<string, (row: WordRow) => string | number | null>;

export type SortKey = keyof typeof SORT_KEYS;

export const listed = (words: readonly string[]) =>
  words.length < 2 ? words.join("") : `${words.slice(0, -1).join(", ")} or ${words[words.length - 1]}`;

const STATUSES = ["todo", "progress", "ready", "approved", "suspended"] as const;
const GENDERS = ["m", "f", "m+f"] as const;
const VERB_FORMS = [
  "I", "II", "III", "IV", "V", "VI", "VII", "VIII", "IX", "X", "XI", "XII", "XIII", "Iq", "IIq", "IIIq", "IVq",
] as const;

function choice(allowed: readonly string[], valuesOf: (row: WordRow) => string | readonly string[]): Field {
  return {
    parse(part) {
      const wanted = part.toLowerCase();
      if (!allowed.some((a) => a.toLowerCase() === wanted)) return `expects ${listed(allowed)}, not "${part}".`;
      return (row) => [valuesOf(row)].flat().some((v) => v.toLowerCase() === wanted);
    },
  };
}

const rankHint = (part: string) => `expects a number, a range such as 1-500 or none, not "${part}".`;

const rank: Field = {
  parse(part) {
    if (part.toLowerCase() === "none") return (row) => row.rank === null;
    const exact = /^(\d+)$/.exec(part);
    if (exact) return (row) => row.rank === Number(exact[1]);
    const range = /^(\d+)-(\d+)$/.exec(part);
    if (range) {
      const [low, high] = [Number(range[1]), Number(range[2])];
      return low <= high ? (row) => row.rank !== null && row.rank >= low && row.rank <= high : rankHint(part);
    }
    return rankHint(part);
  },
};

const rootLetters = (text: string) => fold(text).replace(/\s+/g, "");

const root: Field = {
  parse(part) {
    const wanted = rootLetters(part);
    return (row) => rootLetters(row.root) === wanted;
  },
  pattern: (p) => (row) => p.matches(rootLetters(row.root)),
};

const UNITS = { h: 3_600_000, d: 86_400_000, w: 604_800_000 } as const;

const editedHint = (part: string) => `expects a span such as 7d, 12h or 2w, or none, not "${part}".`;

const edited: Field = {
  parse(part) {
    if (part.toLowerCase() === "none") return (row) => row.edited === null;
    const ago = /^(\d+)([hdw])$/i.exec(part);
    if (!ago) return editedHint(part);
    const span = Number(ago[1]) * UNITS[ago[2].toLowerCase() as keyof typeof UNITS];
    return (row, now) => row.edited !== null && new Date(row.edited).getTime() >= now.getTime() - span;
  },
};

const tag: Field = {
  parse(part) {
    const wanted = part.toLowerCase();
    return (row) => row.tags.some((t) => t.toLowerCase() === wanted);
  },
  pattern: (p) => (row) => row.tags.some((t) => p.matches(fold(t))),
};

export const FIELDS: Record<string, Field> = {
  status: choice(STATUSES, (row) => row.status),
  pos: choice(POS_VALUES, (row) => row.pos),
  rank,
  root,
  gender: choice(GENDERS, (row) => row.gender),
  verbform: choice(VERB_FORMS, (row) => row.verbForm),
  edited,
  tag,
};
