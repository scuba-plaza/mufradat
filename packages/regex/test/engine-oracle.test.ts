import { describe, expect, it } from "vitest";
import { compile } from "../src";
import { createDfa, type DfaExecutor } from "../src/exec/dfa";
import { executorOf } from "./support/pipeline";
import { compare, nativeAnswers, pikeAnswers, referenceAnswers, type Options, type Report } from "./support/oracle";

const OPTIONS = { patterns: 4_000, textsPerPattern: 16, maxLength: 200 };

const engine = (source: string) => {
  const compiled = compile(source);
  return "code" in compiled ? compiled.code : (text: string) => compiled.test(text);
};

function expectAgreement(report: Report) {
  expect(report.disagreements.slice(0, 10)).toEqual([]);
  expect(report.compared).toBeGreaterThanOrEqual(60_000);
  expect(report.skipped / (report.compared + report.skipped)).toBeLessThan(0.01);
  expect(report.matched / report.compared).toBeGreaterThan(0.15);
  expect(report.matched / report.compared).toBeLessThan(0.85);
}

describe("compile() on all of the contract's syntax", () => {
  it("never disagrees with native RegExp on seeded random patterns and texts", () => {
    const options: Options = { ...OPTIONS, scope: "all", seed: 20_240_613, answers: nativeAnswers };
    expectAgreement(compare(engine, options));
  }, 300_000);

  it("never disagrees with the reference matcher on seeded random patterns and texts", () => {
    const options: Options = { ...OPTIONS, scope: "all", seed: 20_240_614, answers: referenceAnswers };
    expectAgreement(compare(engine, options));
  }, 300_000);
});

describe("the lazy DFA on all of the contract's syntax", () => {
  const dfa = (source: string) => {
    const executor = executorOf(source, (program) => createDfa(program));
    return "code" in executor ? executor.code : (text: string) => executor.test(text);
  };

  it("never disagrees with the Pike executor on seeded random patterns and texts", () => {
    const options: Options = { ...OPTIONS, scope: "all", seed: 20_240_615, answers: pikeAnswers };
    expectAgreement(compare(dfa, options));
  }, 300_000);

  it("never disagrees with native RegExp on seeded random patterns and texts", () => {
    const options: Options = { ...OPTIONS, scope: "all", seed: 20_240_616, answers: nativeAnswers };
    expectAgreement(compare(dfa, options));
  }, 300_000);

  it("keeps agreeing with the Pike executor when its cache is tiny, so that it is flushed all the time", () => {
    const made: DfaExecutor[] = [];
    const tiny = (source: string) => {
      const executor = executorOf(source, (program) => {
        const created = createDfa(program, { budget: 40, bailBuilds: Infinity });
        made.push(created);
        return created;
      });
      return "code" in executor ? executor.code : (text: string) => executor.test(text);
    };
    const options: Options = { ...OPTIONS, patterns: 1_500, scope: "all", seed: 20_240_617, answers: pikeAnswers };
    const report = compare(tiny, options);
    expect(report.disagreements.slice(0, 10)).toEqual([]);
    expect(report.compared).toBeGreaterThan(20_000);
    expect(made.reduce((total, executor) => total + executor.stats.flushes, 0)).toBeGreaterThan(5_000);
    expect(made.some((executor) => executor.stats.bailed)).toBe(false);
  }, 300_000);
});

