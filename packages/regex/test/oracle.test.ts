import { describe, expect, it } from "vitest";
import { compilePattern } from "../src";
import { DEFAULT_LIMITS, parse } from "../src/syntax/parse";
import { compare, nativeAnswers, type Report } from "./support/oracle";
import { referenceMatcher } from "./support/reference";

const OPTIONS = { patterns: 4_000, textsPerPattern: 16, maxLength: 200 };

function expectAgreement(report: Report) {
  expect(report.disagreements.slice(0, 10)).toEqual([]);
  expect(report.compared).toBeGreaterThanOrEqual(60_000);
  expect(report.skipped / (report.compared + report.skipped)).toBeLessThan(0.01);
  expect(report.matched / report.compared).toBeGreaterThan(0.15);
  expect(report.matched / report.compared).toBeLessThan(0.85);
}

describe("the current engine against native RegExp on syntax whose meaning does not change", () => {
  it("never disagrees on seeded random patterns and texts", () => {
    const report = compare(
      (source) => {
        const compiled = compilePattern(source);
        return typeof compiled === "string" ? compiled : (text) => compiled.matches(text);
      },
      { ...OPTIONS, scope: "unchanged", seed: 20_240_611, answers: nativeAnswers },
    );
    expectAgreement(report);
  }, 120_000);
});

describe("the parser and the reference matcher against native RegExp on all of the contract's syntax", () => {
  it("never disagree on seeded random patterns and texts", () => {
    const report = compare(
      (source) => {
        const parsed = parse(source, { ...DEFAULT_LIMITS, maxLength: OPTIONS.maxLength });
        return "error" in parsed ? parsed.error.code : referenceMatcher(parsed.tree);
      },
      { ...OPTIONS, scope: "all", seed: 20_240_612, answers: nativeAnswers },
    );
    expectAgreement(report);
  }, 300_000);
});
