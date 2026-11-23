import { describe, expect, it } from "vitest";
import { compile } from "../src";
import { createDfa, type DfaExecutor } from "../src/exec/dfa";
import type { Executor } from "../src/exec/flat";
import { createPike } from "../src/exec/pike";
import type { Inst } from "../src/program/types";
import { parse } from "../src/syntax/parse";
import { compare, nativeAnswers, pikeAnswers, type Subject } from "./support/oracle";
import { executorOf } from "./support/pipeline";
import { referenceMatcher } from "./support/reference";

const OPTIONS = { patterns: 4_000, textsPerPattern: 16, maxLength: 200 };

const through =
  (make: (program: Inst[]) => Executor): Subject =>
  (source) => {
    const executor = executorOf(source, make);
    return "code" in executor ? executor.code : (text) => executor.test(text);
  };

const reference: Subject = (source) => {
  const parsed = parse(source);
  return "error" in parsed ? parsed.error.code : referenceMatcher(parsed.tree);
};

const engine: Subject = (source) => {
  const compiled = compile(source);
  return "code" in compiled ? compiled.code : (text) => compiled.test(text);
};

describe("every engine on all of the contract's syntax", () => {
  it("never disagrees with native RegExp on seeded random patterns and texts", () => {
    const subjects = { reference, pike: through(createPike), "compile()": engine };
    const report = compare(subjects, { ...OPTIONS, seed: 20_240_613, answers: nativeAnswers });
    expect(report.disagreements.slice(0, 10)).toEqual([]);
    expect(report.compared).toBeGreaterThanOrEqual(60_000);
    expect(report.skipped / (report.compared + report.skipped)).toBeLessThan(0.01);
    expect(report.matched / report.compared).toBeGreaterThan(0.15);
    expect(report.matched / report.compared).toBeLessThan(0.85);
  }, 300_000);
});

describe("the lazy DFA with a tiny cache, so that it is flushed all the time", () => {
  it("keeps agreeing with the Pike executor", () => {
    const made: DfaExecutor[] = [];
    const tiny = through((program) => {
      const created = createDfa(program, { budget: 40, bailBuilds: Infinity });
      made.push(created);
      return created;
    });
    const report = compare({ dfa: tiny }, { ...OPTIONS, patterns: 1_500, seed: 20_240_617, answers: pikeAnswers });
    expect(report.disagreements.slice(0, 10)).toEqual([]);
    expect(report.compared).toBeGreaterThan(20_000);
    expect(made.reduce((total, executor) => total + executor.stats.flushes, 0)).toBeGreaterThan(5_000);
    expect(made.some((executor) => executor.stats.bailed)).toBe(false);
  }, 300_000);
});
