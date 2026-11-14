import { renderEngine, renderNative, generate, textsFor, type Scope } from "./generator";
import { nativeMatcher } from "./native";
import { seeded } from "./random";
import { TIMEOUT } from "./watchdog";

export interface Report {
  disagreements: string[];
  compared: number;
  skipped: number;
  matched: number;
}

export interface Options {
  scope: Scope;
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
    const answers = nativeMatcher(renderNative(expression))(texts);
    for (const [index, text] of texts.entries()) {
      const expected = answers[index];
      if (expected === TIMEOUT) {
        report.skipped++;
        continue;
      }
      report.compared++;
      if (expected) report.matched++;
      const actual = subject(text);
      if (actual !== expected) report.disagreements.push(`${JSON.stringify(source)} on ${JSON.stringify(text)}: got ${actual}, native ${expected}`);
    }
  }
  return report;
}
