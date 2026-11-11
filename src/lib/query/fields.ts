import { MISSING_KINDS, POS_VALUES, type WordRow } from "../types";
import { fold } from "./fuzzy";

export type Test = (row: WordRow, now: Date) => boolean;

export interface Field {
  parse(part: string): Test | string;
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

const rankHint = (part: string) => `expects a number, a range such as 1-500, >2000, <=100 or none, not "${part}".`;

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
    const compare = /^(>=|<=|>|<)(\d+)$/.exec(part);
    if (!compare) return rankHint(part);
    const limit = Number(compare[2]);
    const holds = { ">": (r: number) => r > limit, ">=": (r: number) => r >= limit, "<": (r: number) => r < limit, "<=": (r: number) => r <= limit }[
      compare[1] as ">" | ">=" | "<" | "<="
    ];
    return (row) => row.rank !== null && holds(row.rank);
  },
};

const rootLetters = (text: string) => fold(text).replace(/\s+/g, "");

const root: Field = {
  parse(part) {
    const wanted = rootLetters(part);
    return (row) => rootLetters(row.root) === wanted;
  },
};

const UNITS = { h: 3_600_000, d: 86_400_000, w: 604_800_000 } as const;

function day(text: string): Date | null {
  const found = /^(\d{4})-(\d{2})-(\d{2})$/.exec(text);
  if (!found) return null;
  const [year, month, date] = [Number(found[1]), Number(found[2]), Number(found[3])];
  const parsed = new Date(year, month - 1, date);
  return parsed.getFullYear() === year && parsed.getMonth() === month - 1 && parsed.getDate() === date ? parsed : null;
}

const editedHint = (part: string) =>
  `expects 7d, 12h, 2w, a date such as 2026-11-01, a range such as 2026-11-01.. or ..2026-11-01, or none, not "${part}".`;

const edited: Field = {
  parse(part) {
    if (part.toLowerCase() === "none") return (row) => row.edited === null;
    const ago = /^(\d+)([hdw])$/i.exec(part);
    if (ago) {
      const span = Number(ago[1]) * UNITS[ago[2].toLowerCase() as keyof typeof UNITS];
      return (row, now) => row.edited !== null && new Date(row.edited).getTime() >= now.getTime() - span;
    }
    const [first, last = first, ...extra] = part.split("..");
    const from = first === "" ? null : day(first);
    const to = last === "" ? null : day(last);
    if (extra.length > 0 || (first === "" && last === "") || (first !== "" && !from) || (last !== "" && !to)) return editedHint(part);
    if (from && to && from.getTime() > to.getTime()) return editedHint(part);
    const until = to && new Date(to.getFullYear(), to.getMonth(), to.getDate() + 1);
    return (row) => {
      if (row.edited === null) return false;
      const time = new Date(row.edited).getTime();
      return (!from || time >= from.getTime()) && (!until || time < until.getTime());
    };
  },
};

function matchesGlob(pattern: string, text: string): boolean {
  let p = 0;
  let t = 0;
  let star = -1;
  let resume = 0;
  while (t < text.length) {
    if (pattern[p] === "*") {
      star = p++;
      resume = t;
    } else if (p < pattern.length && pattern[p] === text[t]) {
      p++;
      t++;
    } else if (star >= 0) {
      p = star + 1;
      t = ++resume;
    } else {
      return false;
    }
  }
  while (pattern[p] === "*") p++;
  return p === pattern.length;
}

const tag: Field = {
  parse(part) {
    const wanted = part.toLowerCase();
    return (row) => row.tags.some((t) => matchesGlob(wanted, t.toLowerCase()));
  },
};

export const FIELDS: Record<string, Field> = {
  status: choice(STATUSES, (row) => row.status),
  pos: choice(POS_VALUES, (row) => row.pos),
  rank,
  root,
  gender: choice(GENDERS, (row) => row.gender),
  verbform: choice(VERB_FORMS, (row) => row.verbForm),
  missing: choice(MISSING_KINDS, (row) => row.missing),
  has: choice(["error"], (row) => (row.error ? "error" : [])),
  example: choice(["mistral", "mine"], (row) => (row.exampleBy === "user" ? "mine" : row.exampleBy)),
  edited,
  tag,
};
