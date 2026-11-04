import type { Note } from "./types";
import { POS_VALUES } from "./types";

const ARABIC = /[؀-ۿݐ-ݿ]/;
const LATIN = /[A-Za-z]/;

const hasArabic = (s: string) => ARABIC.test(s);
const hasLatin = (s: string) => LATIN.test(s);

interface ExampleParts {
  before: string;
  target: string;
  after: string;
}

const EXAMPLE_MARKUP_ERROR =
  "The example must contain exactly one pair of [square brackets] around the target word.";

export function splitExample(s: string): ExampleParts | null {
  const count = (c: string) => s.split(c).length - 1;
  if (count("[") !== 1 || count("]") !== 1) return null;
  const open = s.indexOf("[");
  const close = s.indexOf("]");
  if (close < open) return null;
  const target = s.slice(open + 1, close);
  if (target.trim() === "") return null;
  return { before: s.slice(0, open), target, after: s.slice(close + 1) };
}

export function plainExample(s: string): string {
  const p = splitExample(s);
  return p ? p.before + p.target + p.after : s.replace(/[[\]]/g, "");
}

function wordCount(s: string): number {
  return plainExample(s).split(/\s+/).filter(Boolean).length;
}

/** Everything wrong with the sentence of an example from Mistral, whichever word is marked in it: length and script. */
export function sentenceProblems(sentence: string): string[] {
  const problems: string[] = [];
  const words = wordCount(sentence);
  if (words < 3 || words > 9) problems.push(`The example has ${words} words; it must have 3 to 9.`);
  if (!hasArabic(sentence)) problems.push("The example has no Arabic text.");
  if (hasLatin(sentence)) problems.push("The Arabic example contains Latin letters.");
  return problems;
}

const VOWEL_MARK = /[ً-ٰٟ]/;
const WORD = /[^\s،؛؟.,;:!?"'()«»[\]]+/g;
/** A word by its letters alone: no vowels, tatweel or article, so that كِتَابٌ and الْكِتَابِ are the same. */
const letters = (s: string) => skeleton(s).replace(/^ال/, "");

/**
 * The sentence with [brackets] around the word the card is about, which Mistral names as it stands in the sentence.
 * The word is looked for as it was named, then attached to the next word (بِالْقَلَمِ), then by its letters alone
 * (another vowel, case ending or article). It returns null when the word is in none of these places or in several
 * of them: it never guesses, and the person marks the word.
 */
export function markTarget(sentence: string, named: string): string | null {
  const target = named.replace(/ـ/g, "").trim();
  if (target === "") return null;
  const mark = (start: number, length: number) =>
    `${sentence.slice(0, start)}[${sentence.slice(start, start + length)}]${sentence.slice(start + length)}`;

  if (/\s/.test(target)) {
    const at = sentence.indexOf(target);
    return at >= 0 && sentence.indexOf(target, at + 1) < 0 ? mark(at, target.length) : null;
  }
  const words = [...sentence.matchAll(WORD)].map((m) => ({ text: m[0], start: m.index }));
  // Each rule gives how much of a word is the target, or 0. The first rule that finds anything decides.
  const rules: ((word: string) => number)[] = [
    (w) => (w === target ? w.length : 0),
    (w) => (w.length > target.length && w.startsWith(target) && !VOWEL_MARK.test(w[target.length]) ? target.length : 0),
    (w) => (letters(w) === letters(target) ? w.length : 0),
  ];
  for (const rule of rules) {
    const hits = words.flatMap((w) => (rule(w.text) > 0 ? [{ start: w.start, length: rule(w.text) }] : []));
    if (hits.length === 1) return mark(hits[0].start, hits[0].length);
    if (hits.length > 1) return null;
  }
  return null;
}

/** What stops a card from being approved. */
export function approvalProblems(n: Note): string[] {
  const problems: string[] = [];
  if (!hasArabic(n.lemma)) problems.push("The headword must be Arabic text.");
  if (n.english.trim() === "") problems.push("The English meaning is empty.");
  if (!POS_VALUES.includes(n.pos)) problems.push("The part of speech is not valid.");
  for (const f of n.forms) {
    if (f.label.trim() === "") problems.push(`The form ${f.arabic} has no label.`);
    if (!hasArabic(f.arabic)) problems.push(`The form "${f.arabic}" must be Arabic text.`);
  }
  if (n.example.trim() !== "") {
    if (!splitExample(n.example)) problems.push(EXAMPLE_MARKUP_ERROR);
    else if (!hasArabic(plainExample(n.example))) problems.push("The example has no Arabic text.");
    if (n.exampleEn.trim() === "") problems.push("The example has no English translation.");
  }
  return problems;
}

/** Arabic without vowel marks, tatweel, or alef/yeh variants, for matching typed searches. */
export function skeleton(s: string): string {
  return s
    .replace(/[ً-ٰٟـ]/g, "")
    .replace(/[أإآٱ]/g, "ا")
    .replace(/ى/g, "ي");
}
