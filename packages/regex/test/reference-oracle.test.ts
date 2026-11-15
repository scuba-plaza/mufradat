import { describe, expect, it } from "vitest";
import { DEFAULT_LIMITS, parse } from "../src/syntax/parse";
import { compare, nativeAnswers } from "./support/oracle";
import { referenceMatcher } from "./support/reference";

describe("the parser and the reference matcher against native RegExp on all of the contract's syntax", () => {
  it("never disagree on seeded random patterns and texts", () => {
    const report = compare(
      (source) => {
        const parsed = parse(source, { ...DEFAULT_LIMITS, maxLength: 200 });
        return "error" in parsed ? parsed.error.code : referenceMatcher(parsed.tree);
      },
      { patterns: 4_000, textsPerPattern: 16, maxLength: 200, seed: 20_240_612, answers: nativeAnswers },
    );
    expect(report.disagreements.slice(0, 10)).toEqual([]);
    expect(report.compared).toBeGreaterThanOrEqual(60_000);
    expect(report.skipped / (report.compared + report.skipped)).toBeLessThan(0.01);
    expect(report.matched / report.compared).toBeGreaterThan(0.15);
    expect(report.matched / report.compared).toBeLessThan(0.85);
  }, 300_000);
});
