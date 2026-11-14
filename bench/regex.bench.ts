import { it } from "vitest";
import { compilePattern, type Pattern } from "@mufradat/regex";
import { rankedWords, wiktionaryWords } from "../src/lib/lexicon";
import { fold } from "../src/lib/query/fuzzy";
import { seeded } from "../packages/regex/test/support/random";

interface Engine {
  name: string;
  compile(pattern: string): Pattern | string;
}

interface Corpus {
  name: string;
  texts: string[];
}

const ENGINES: Engine[] = [{ name: "current", compile: compilePattern }];

const SLOW_MS = 2_000;
const WARMUP_MS = 1_000;
const WARMUP_RUNS = 3;
const MIN_MS = 3_000;
const MIN_RUNS = 7;
const MAX_RUNS = 200;

const write = (line: string) => process.stdout.write(`${line}\n`);
const group = (n: number) => n.toLocaleString("en-US");

function lexicon(): Corpus {
  const rows = [...rankedWords(), ...wiktionaryWords().filter((word) => word.rank === 0)];
  return { name: "lexicon", texts: rows.flatMap((row) => [fold(row.lemma), fold(row.english)]) };
}

function randomAB(length: number): Corpus {
  const random = seeded(1);
  let text = "";
  for (let k = 0; k < length; k++) text += random.chance(0.5) ? "a" : "b";
  return { name: `random a/b, ${group(length)} characters`, texts: [text] };
}

const median = (samples: number[]): number => {
  const sorted = [...samples].sort((a, b) => a - b);
  return sorted[Math.floor(sorted.length / 2)];
};

interface Measured {
  median: number;
  min: number;
  max: number;
  runs: number;
  matching: number;
}

function measure(run: () => number): Measured {
  const timed = () => {
    const start = performance.now();
    const matching = run();
    return { ms: performance.now() - start, matching };
  };
  const first = timed();
  if (first.ms > SLOW_MS) return { median: first.ms, min: first.ms, max: first.ms, runs: 1, matching: first.matching };
  for (let runs = 1, spent = first.ms; runs < WARMUP_RUNS || spent < WARMUP_MS; runs++) spent += timed().ms;
  const samples: number[] = [];
  let matching = first.matching;
  for (let spent = 0; samples.length < MAX_RUNS && (samples.length < MIN_RUNS || spent < MIN_MS); ) {
    const sample = timed();
    samples.push(sample.ms);
    spent += sample.ms;
    matching = sample.matching;
  }
  return { median: median(samples), min: Math.min(...samples), max: Math.max(...samples), runs: samples.length, matching };
}

const ms = (value: number) => value.toFixed(value < 100 ? 1 : 0).padStart(9);

it("prints one table per engine over the real lexicon", () => {
  const words = lexicon();
  const hostile = randomAB(200_000);
  const characters = words.texts.reduce((sum, text) => sum + Array.from(text).length, 0);
  const cases: { pattern: string; corpus: Corpus }[] = [
    { pattern: "kitab", corpus: words },
    { pattern: "\\w+ing", corpus: words },
    { pattern: "[a-z]{3,5}ing$", corpus: words },
    { pattern: "(.{20,20}){0,20}\\u0001", corpus: words },
    { pattern: "(.{100,100}){0,100}\\u0001", corpus: words },
    { pattern: "a.{18,18}c", corpus: hostile },
  ];
  write(`lexicon: ${group(words.texts.length)} texts, ${group(characters)} characters`);
  for (const engine of ENGINES) {
    write("");
    write(`engine: ${engine.name}`);
    write(`${"pattern".padEnd(28)}${"corpus".padEnd(8)}${"median ms".padStart(10)}${"min".padStart(9)}${"max".padStart(9)}${"runs".padStart(6)}${"matching".padStart(10)}`);
    for (const { pattern, corpus } of cases) {
      const result = measure(() => {
        const compiled = engine.compile(pattern);
        if (typeof compiled === "string") throw new Error(`${pattern}: ${compiled}`);
        let matching = 0;
        for (const text of corpus.texts) if (compiled.matches(text)) matching++;
        return matching;
      });
      const label = corpus === words ? "lexicon" : "a/b";
      write(`${pattern.padEnd(28)}${label.padEnd(8)}${ms(result.median)} ${ms(result.min)}${ms(result.max)}${String(result.runs).padStart(6)}${String(result.matching).padStart(10)}`);
    }
  }
});
