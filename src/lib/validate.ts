import type { Note } from "./types";
import { POS_VALUES } from "./types";

const ARABIC = /[؀-ۿݐ-ݿ]/;
const LATIN = /[A-Za-z]/;
/** An Arabic letter: not a vowel mark, the tatweel or a digit. */
const LETTER = /[\u0621-\u063A\u0641-\u064A\u0671-\u06D3]/;
const COUNTED = /[\p{L}\p{N}]/u;

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

const VOWEL_MARK = /[\u064B-\u065F\u0670]/;
/** A vowel the letter before it is read with: tanwin, fatha, damma, kasra, sukun or the dagger alif. A shadda alone is not one. */
const VOWEL = /[\u064B-\u0650\u0652\u0670]/;
const SPOKEN_WORD = /[^\s،؛؟.,;:!?"'()«»—–…“”‘’-]+/g;

/**
 * What is wrong with the sentence of the agent's answer, by the rules of its system prompt: written in Arabic,
 * 2 to 12 words, no square brackets (the app marks the card's word with them), and a vowel mark on every word. Whether
 * a word is vocalized completely, and every other rule (collocations, history, grammar), cannot be checked here.
 */
export function sentenceProblems(sentence: string): string[] {
  const problems: string[] = [];
  const words = (sentence.match(SPOKEN_WORD) ?? []).filter((w) => COUNTED.test(w));
  if (words.length < 2 || words.length > 12) {
    problems.push(`The sentence has ${words.length} ${words.length === 1 ? "word" : "words"}; it must have 2 to 12.`);
  }
  if (!hasArabic(sentence)) problems.push("The sentence has no Arabic text.");
  if (hasLatin(sentence)) problems.push("The Arabic sentence contains Latin letters.");
  if (/[[\]]/.test(sentence)) problems.push("The sentence contains square brackets.");
  const bare = words.filter((w) => LETTER.test(w) && !VOWEL.test(w));
  if (bare.length > 0) problems.push(`The sentence has words without a vowel mark: ${bare.join(" ")}.`);
  return problems;
}

/** What is wrong with the translation of the agent's answer: it must not be in Arabic. */
export function translationProblems(translation: string): string[] {
  return hasArabic(translation) ? ["The translation contains Arabic text."] : [];
}

const NON_LETTER = /[\u064B-\u065F\u0670\u0640]/;
const WORD = /[^\s،؛؟.,;:!?"'()«»[\]]+/g;
/** A word by its letters alone: no vowels, tatweel or article, so that كِتَابٌ and الْكِتَابِ are the same. */
const letters = (s: string) => skeleton(s).replace(/^ال/, "");

type Found = { start: number; end: number } | "many" | null;

const wrap = (s: string, start: number, end: number) => `${s.slice(0, start)}[${s.slice(start, end)}]${s.slice(end)}`;
const marked = (s: string, found: Found) => (found && found !== "many" ? wrap(s, found.start, found.end) : null);

function findTarget(sentence: string, named: string): Found {
  const target = named.replace(/ـ/g, "").trim();
  if (target === "") return null;

  if (/\s/.test(target)) {
    const at = sentence.indexOf(target);
    if (at < 0) return null;
    return sentence.indexOf(target, at + 1) < 0 ? { start: at, end: at + target.length } : "many";
  }
  const words = [...sentence.matchAll(WORD)].map((m) => ({ text: m[0], start: m.index }));
  // Each rule gives how much of a word is the target, or 0. The first rule that finds anything decides.
  const rules: ((word: string) => number)[] = [
    (w) => (w === target ? w.length : 0),
    (w) => (w.length > target.length && w.startsWith(target) && !VOWEL_MARK.test(w[target.length]) ? target.length : 0),
    (w) => (letters(w) === letters(target) ? w.length : 0),
  ];
  for (const rule of rules) {
    const hits = words.flatMap((w) => (rule(w.text) > 0 ? [{ start: w.start, end: w.start + rule(w.text) }] : []));
    if (hits.length === 1) return hits[0];
    if (hits.length > 1) return "many";
  }
  return null;
}

const PREFIXES = ["", "و", "ف"].flatMap((conjunction) =>
  ["", "ب", "ك", "ل"].flatMap((preposition) => ["", "ال"].map((article) => conjunction + preposition + article)).concat(`${conjunction}لل`),
);
const SUFFIXES = ["", "ه", "ها", "هم", "هن", "هما", "ك", "كم", "كن", "كما", "ني", "نا", "ي"];

function stemSpan(word: string, stem: string): { start: number; end: number } | null {
  const letterAt = [...word].flatMap((c, i) => (NON_LETTER.test(c) ? [] : [{ c: skeleton(c), i }]));
  const skeletal = letterAt.map((l) => l.c).join("");
  const prefix = PREFIXES.find((p) => skeletal.startsWith(p + stem) && SUFFIXES.includes(skeletal.slice(p.length + stem.length)));
  if (prefix === undefined) return null;
  const article = prefix.endsWith("ال") ? 2 : 0;
  const first = letterAt[prefix.length - article].i;
  let end = letterAt[prefix.length + stem.length - 1].i + 1;
  while (end < word.length && NON_LETTER.test(word[end])) end++;
  return { start: first, end };
}

function findStem(sentence: string, named: string): Found {
  const stem = letters(named);
  if (stem === "") return null;
  const hits = [...sentence.matchAll(WORD)].flatMap((m) => {
    const span = stemSpan(m[0], stem);
    return span ? [{ start: m.index + span.start, end: m.index + span.end }] : [];
  });
  return hits.length > 1 ? "many" : (hits[0] ?? null);
}

export function autoMark(sentence: string, words: string[]): string | null {
  for (const word of words) {
    const found = findTarget(sentence, word) ?? findStem(sentence, word);
    if (found === "many") return null;
    if (found) return marked(sentence, found);
  }
  return null;
}

export function markSelection(draft: string, start: number, end: number): string {
  const plain = plainExample(draft);
  let from = plainExample(draft.slice(0, start)).length;
  let to = plainExample(draft.slice(0, end)).length;
  while (from < to && /\s/.test(plain[from])) from++;
  while (to > from && /\s/.test(plain[to - 1])) to--;
  if (from === to || (from === 0 && to === plain.length)) return draft;
  const parts = splitExample(draft);
  if (parts && from === parts.before.length && to === from + parts.target.length) return draft;
  return wrap(plain, from, to);
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
    .replace(/[\u064B-\u065F\u0670\u0640]/g, "")
    .replace(/[أإآٱ]/g, "ا")
    .replace(/ى/g, "ي");
}
