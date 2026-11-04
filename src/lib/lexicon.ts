import fs from "node:fs";
import path from "node:path";
import type { NewNote, POS } from "./types";
import { skeleton } from "./validate";

interface Sense { gloss: string; msa: boolean }
interface Entry {
  pos: string; canonical: string; gender?: string; root?: string; verb_form?: string;
  non_past?: string; verbal_nouns?: string[]; plurals?: string[]; feminine?: string;
  feminine_plurals?: string[]; elative?: string; senses: Sense[];
}
interface RankRow { rank: number; id: string; pos: string; gloss: string }

const POS_FROM_WIKTIONARY: Record<string, POS> = {
  noun: "noun", verb: "verb", adj: "adj", adv: "adv", prep: "prep", conj: "conj", particle: "particle",
  pron: "pron", num: "num", intj: "intj", phrase: "phrase", det: "det", article: "det", prep_phrase: "prep",
};

interface Data {
  ranking: RankRow[];
  rankOfLemma: Map<string, RankRow>;
  lexicon: Map<string, Entry[]>;
  bySkeleton: Map<string, string[]>;
}

let cached: Data | null = null;

function load(): Data {
  if (cached) return cached;
  const dir = path.join(process.cwd(), "data");
  const ranking = fs.readFileSync(path.join(dir, "ranking.tsv"), "utf8").trim().split("\n").slice(1).map((line) => {
    const [rank, id, pos, gloss] = line.split("\t");
    return { rank: Number(rank), id, pos, gloss };
  });
  const lexicon = new Map<string, Entry[]>();
  const bySkeleton = new Map<string, string[]>();
  for (const line of fs.readFileSync(path.join(dir, "lexicon.jsonl"), "utf8").trim().split("\n")) {
    const rec = JSON.parse(line) as { id: string; entries: Entry[] };
    const entries = rec.entries
      .map((e) => ({ ...e, senses: e.senses.filter((s) => s.msa) }))
      .filter((e) => e.senses.length > 0 && POS_FROM_WIKTIONARY[e.pos]);
    if (entries.length === 0) continue;
    lexicon.set(rec.id, entries);
    const key = skeleton(rec.id);
    bySkeleton.set(key, [...(bySkeleton.get(key) ?? []), rec.id]);
  }
  const rankOfLemma = new Map<string, RankRow>();
  for (const r of ranking) if (!rankOfLemma.has(r.id)) rankOfLemma.set(r.id, r);
  cached = { ranking, rankOfLemma, lexicon, bySkeleton };
  return cached;
}

export const rankingSize = () => load().ranking.length;

export interface Word {
  rank: number;
  lemma: string;
  english: string;
}

/** The frequency list as Wiktionary gives it. */
export const rankedWords = (): Word[] => load().ranking.map((r) => ({ rank: r.rank, lemma: r.id, english: r.gloss }));

function forms(e: Entry): { label: string; arabic: string }[] {
  const out: { label: string; arabic: string }[] = [];
  const add = (label: string, list: string[] | string | undefined) => {
    for (const arabic of [list ?? []].flat()) out.push({ label, arabic });
  };
  add("pres.", e.non_past);
  add("masdar", e.verbal_nouns);
  add("fem.", e.feminine);
  add("pl.", e.plurals);
  add("fem. pl.", e.feminine_plurals);
  add("elative", e.elative);
  return out;
}

function bareCard(lemma: string, e: Entry, rankRow: RankRow | undefined, gloss?: string): NewNote {
  const pos = POS_FROM_WIKTIONARY[e.pos];
  const ranked = rankRow && POS_FROM_WIKTIONARY[rankRow.pos.split(",")[0]] === pos;
  return {
    lemma: e.canonical || lemma,
    english: gloss ?? e.senses[0].gloss,
    pos,
    gender: e.gender ?? "",
    root: e.root ?? "",
    verbForm: e.verb_form ?? "",
    rank: ranked ? rankRow.rank : 0,
    forms: forms(e),
  };
}

/** The bare card for a rank in the frequency list, from its first listed part of speech. */
export function cardForRank(rank: number): NewNote | null {
  const d = load();
  const row = d.ranking[rank - 1];
  if (!row) return null;
  const want = POS_FROM_WIKTIONARY[row.pos.split(",")[0]];
  const entry = d.lexicon.get(row.id)?.find((e) => POS_FROM_WIKTIONARY[e.pos] === want);
  return entry ? bareCard(row.id, entry, row, row.gloss) : null;
}

/** Bare cards for every Wiktionary entry whose lemma matches the typed word, ignoring vowels. */
export function lookup(query: string): NewNote[] {
  const d = load();
  const q = skeleton(query.trim());
  if (q === "") return [];
  const out: NewNote[] = [];
  for (const id of d.bySkeleton.get(q) ?? []) {
    const seen = new Set<POS>();
    for (const e of d.lexicon.get(id)!) {
      const pos = POS_FROM_WIKTIONARY[e.pos];
      if (seen.has(pos)) continue;
      seen.add(pos);
      out.push(bareCard(id, e, d.rankOfLemma.get(id)));
    }
  }
  return out;
}
