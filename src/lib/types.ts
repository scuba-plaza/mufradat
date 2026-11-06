export const POS_VALUES = [
  "noun", "verb", "adj", "adv", "prep", "conj", "particle", "pron", "num", "intj", "phrase", "det",
] as const;
export type POS = (typeof POS_VALUES)[number];

export const POS_LABELS: Record<POS, string> = {
  noun: "noun", verb: "verb", adj: "adjective", adv: "adverb", prep: "preposition",
  conj: "conjunction", particle: "particle", pron: "pronoun", num: "number",
  intj: "interjection", phrase: "phrase", det: "determiner",
};

export type CardStatus = "draft" | "incomplete" | "approved";
export type WordStatus = "todo" | "progress" | "ready" | "approved";
export type AudioSource = "tts" | "user";

/** "word", "example", or a form id. */
export type Slot = string;

export interface Form {
  id: string;
  label: string;
  arabic: string;
}

export interface Clip {
  key: string | null;
  source: AudioSource | null;
  /** The Arabic text the clip was made for; the clip is outdated once the field differs. */
  text: string;
  error: string | null;
  outdated: boolean;
}

export interface Note {
  id: string;
  lemma: string;
  english: string;
  pos: POS;
  gender: string;
  root: string;
  verbForm: string;
  hint: string;
  example: string;
  exampleEn: string;
  exampleError: string | null;
  rank: number;
  status: CardStatus;
  lemmaSource: "wiktionary" | "user";
  exampleSource: "mistral" | "user" | "";
  createdAt: string;
  updatedAt: string;
  forms: Form[];
  audio: Record<Slot, Clip>;
}

export interface NewNote {
  lemma: string;
  english: string;
  pos: POS;
  gender?: string;
  root?: string;
  verbForm?: string;
  rank?: number;
  forms?: { label: string; arabic: string }[];
}

export interface NoteEdit {
  lemma?: string;
  english?: string;
  pos?: POS;
  gender?: string;
  root?: string;
  verbForm?: string;
  hint?: string;
  example?: string;
  exampleEn?: string;
  /** Entries with an id keep their audio; entries without one are new. Omitted entries are removed. */
  forms?: { id?: string; label: string; arabic: string }[];
}

export const MISSING_KINDS = ["example", "translation", "target", "audio"] as const;
export type MissingKind = (typeof MISSING_KINDS)[number];

/** One word of the list: a card, a frequency word that has none yet, or a Wiktionary entry that matched a search. */
export interface WordRow {
  id: string | null;
  rank: number | null;
  lemma: string;
  english: string;
  pos: POS;
  gender: string;
  root: string;
  verbForm: string;
  status: WordStatus;
  missing: MissingKind[];
  error: boolean;
  exampleBy: "mistral" | "user" | "";
  edited: string | null;
}
