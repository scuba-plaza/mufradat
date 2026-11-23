import { createPike } from "../../src/exec/pike";
import { generate, renderEngine, renderNative, textsFor, type Expression } from "./generator";
import { nativeMatcher } from "./native";
import { build } from "./pipeline";
import { seeded } from "./random";
import { TIMEOUT } from "./watchdog";

export interface Report {
  disagreements: string[];
  compared: number;
  skipped: number;
  matched: number;
}

export type Answers = (source: string, expression: Expression, texts: readonly string[]) => (boolean | typeof TIMEOUT)[];

export type Subject = (source: string) => ((text: string) => boolean) | string;

export const nativeAnswers: Answers = (_source, expression, texts) => nativeMatcher(renderNative(expression))(texts);

export const pikeAnswers: Answers = (source, _expression, texts) => {
  const built = build(source);
  if ("error" in built) throw new Error(`${source} was refused with ${built.error.code}`);
  const executor = createPike(built.program);
  return texts.map((text) => executor.test(text));
};

export interface Options {
  answers: Answers;
  seed: number;
  patterns: number;
  textsPerPattern: number;
  maxLength: number;
}

export function compare(subjects: Record<string, Subject>, options: Options): Report {
  const random = seeded(options.seed);
  const report: Report = { disagreements: [], compared: 0, skipped: 0, matched: 0 };
  const named = Object.entries(subjects);
  for (let k = 0; k < options.patterns; k++) {
    const expression = generate(random, options.maxLength);
    const source = renderEngine(expression);
    const matchers: [string, (text: string) => boolean][] = [];
    for (const [name, subject] of named) {
      const matcher = subject(source);
      if (typeof matcher === "string") report.disagreements.push(`${name}: ${JSON.stringify(source)} was refused: ${matcher}`);
      else matchers.push([name, matcher]);
    }
    if (matchers.length < named.length) continue;
    const texts = textsFor(random, expression, options.textsPerPattern);
    const answers = options.answers(source, expression, texts);
    for (const [index, text] of texts.entries()) {
      const expected = answers[index];
      if (expected === TIMEOUT) {
        report.skipped++;
        continue;
      }
      report.compared++;
      if (expected) report.matched++;
      for (const [name, matcher] of matchers) {
        const actual = matcher(text);
        if (actual !== expected) report.disagreements.push(`${name}: ${JSON.stringify(source)} on ${JSON.stringify(text)}: got ${actual}, expected ${expected}`);
      }
    }
  }
  return report;
}
