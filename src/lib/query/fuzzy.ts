import { skeleton } from "../validate";

export const fold = (text: string): string => skeleton(text.toLowerCase()).replace(/ة/g, "ه");

const wordsOf = (text: string) => text.split(/[^\p{L}\p{N}]+/u).filter(Boolean);

const typoBudget = (length: number) => (length <= 3 ? 0 : length <= 6 ? 1 : 2);

function distance(a: string, b: string): number {
  const d: number[][] = Array.from({ length: a.length + 1 }, (_, i) => [i, ...Array<number>(b.length).fill(0)]);
  for (let j = 1; j <= b.length; j++) d[0][j] = j;
  for (let i = 1; i <= a.length; i++) {
    for (let j = 1; j <= b.length; j++) {
      d[i][j] = Math.min(d[i - 1][j] + 1, d[i][j - 1] + 1, d[i - 1][j - 1] + (a[i - 1] === b[j - 1] ? 0 : 1));
      if (i > 1 && j > 1 && a[i - 1] === b[j - 2] && a[i - 2] === b[j - 1]) d[i][j] = Math.min(d[i][j], d[i - 2][j - 2] + 1);
    }
  }
  return d[a.length][b.length];
}

export function closeness(term: string, text: string, phrase = false): number {
  const wanted = fold(term);
  if (wanted === "") return Infinity;
  const folded = fold(text);
  const words = wordsOf(folded);
  if (folded === wanted) return 0;
  if (words.includes(wanted)) return 1;
  if (words.some((word) => word.startsWith(wanted))) return 2;
  if (folded.includes(wanted)) return 3;
  const budget = phrase ? 0 : typoBudget(wanted.length);
  if (budget === 0) return Infinity;
  let nearest = Infinity;
  for (const word of words) {
    if (Math.abs(word.length - wanted.length) <= budget) nearest = Math.min(nearest, distance(wanted, word));
  }
  return nearest <= budget ? 4 + nearest : Infinity;
}
