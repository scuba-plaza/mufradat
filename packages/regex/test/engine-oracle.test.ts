import { describe, expect, it } from "vitest";
import { compile } from "../src";
import { compare, nativeAnswers, referenceAnswers, type Options, type Report } from "./support/oracle";

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
