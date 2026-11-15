import { createPike } from "../../src/exec/pike";
import { parse } from "../../src/syntax/parse";
import { generate, renderEngine, renderNative, textsFor, type Expression, type Scope } from "./generator";
import { nativeMatcher } from "./native";
import { build } from "./pipeline";
import { seeded } from "./random";
import { referenceMatcher } from "./reference";
import { TIMEOUT } from "./watchdog";

export interface Report {
  disagreements: string[];
  compared: number;
  skipped: number;
  matched: number;
}

export type Answers = (source: string, expression: Expression, texts: readonly string[]) => (boolean | typeof TIMEOUT)[];

export const nativeAnswers: Answers = (_source, expression, texts) => nativeMatcher(renderNative(expression))(texts);

export const referenceAnswers: Answers = (source, _expression, texts) => {
  const parsed = parse(source);
  if ("error" in parsed) throw new Error(`${source} was refused with ${parsed.error.code}`);
  const matcher = referenceMatcher(parsed.tree);
  return texts.map(matcher);
};

export const pikeAnswers: Answers = (source, _expression, texts) => {
  const built = build(source);
  if ("error" in built) throw new Error(`${source} was refused with ${built.error.code}`);
  const executor = createPike(built.program);
  return texts.map((text) => executor.test(text));
};

export interface Options {
  scope: Scope;
  answers: Answers;
  seed: number;
  patterns: number;
  textsPerPattern: number;
  maxLength: number;
}

export function compare(compile: (source: string) => ((text: string) => boolean) | string, options: Options): Report {
  const random = seeded(options.seed);
  const report: Report = { disagreements: [], compared: 0, skipped: 0, matched: 0 };
  for (let k = 0; k < options.patterns; k++) {
    const expression = generate(random, options.scope, options.maxLength);
    const source = renderEngine(expression);
    const subject = compile(source);
    if (typeof subject === "string") {
      report.disagreements.push(`${JSON.stringify(source)} was refused: ${subject}`);
      continue;
    }
    const texts = textsFor(random, options.scope, expression, options.textsPerPattern);
    const answers = options.answers(source, expression, texts);
    for (const [index, text] of texts.entries()) {
      const expected = answers[index];
      if (expected === TIMEOUT) {
        report.skipped++;
        continue;
      }
      report.compared++;
      if (expected) report.matched++;
      const actual = subject(text);
      if (actual !== expected) report.disagreements.push(`${JSON.stringify(source)} on ${JSON.stringify(text)}: got ${actual}, expected ${expected}`);
    }
  }
  return report;
}
