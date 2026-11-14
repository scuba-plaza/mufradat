import { describe, expect, it } from "vitest";
import { compilePattern } from "../src";
import { nativeMatcher } from "./support/native";
import { seeded } from "./support/random";
import { generate, renderEngine, renderNative, textsFor, type Expression } from "./support/syntax";
import { TIMEOUT } from "./support/watchdog";

const MAX_PATTERN_LENGTH = 200;
const PATTERNS = 4_000;
const TEXTS_PER_PATTERN = 16;
const SEED = 20_240_611;

describe("the native RegExp oracle", () => {
  it("gives up on a native pattern that backtracks without end, and says so", () => {
    const stuck = nativeMatcher("^(?:\\u{61}+)+$");
    expect(stuck(["a".repeat(40) + "!", "aaa", "b"])).toEqual([TIMEOUT, true, false]);
  });

  it("writes one expression as an engine pattern and as native source", () => {
    const expression: Expression = [
      [
        { kind: "start" },
        { kind: "repeat", body: { kind: "literal", code: 0x2a, escaped: false }, min: 2, max: null, counted: true },
        { kind: "class", negated: true, items: [{ kind: "range", from: 0x61, to: 0x63 }, { kind: "perl", letter: "W" }, { kind: "literal", code: 0x2d }] },
        { kind: "group", body: [[{ kind: "any" }, { kind: "perl", letter: "w" }], [{ kind: "literal", code: 0x7c, escaped: true }]] },
        { kind: "end" },
      ],
    ];
    expect(renderEngine(expression)).toBe("^\\*{2,}[^a-c\\W\\-](.\\w|\\|)$");
    expect(renderNative(expression)).toBe("^\\u{2a}{2,}[^\\u{61}-\\u{63}[^\\p{L}\\p{N}_]\\u{2d}](?:[^][\\p{L}\\p{N}_]|\\u{7c})$");
  });
});

describe("the engine against native RegExp on syntax both understand", () => {
  it("never disagrees on seeded random patterns and texts", () => {
    const random = seeded(SEED);
    const disagreements: string[] = [];
    let compared = 0;
    let skipped = 0;
    let matched = 0;
    for (let k = 0; k < PATTERNS; k++) {
      const expression = generate(random, MAX_PATTERN_LENGTH);
      const source = renderEngine(expression);
      const compiled = compilePattern(source);
      if (typeof compiled === "string") {
        disagreements.push(`${JSON.stringify(source)} was refused: ${compiled}`);
        continue;
      }
      const native = nativeMatcher(renderNative(expression));
      const texts = textsFor(random, expression, TEXTS_PER_PATTERN);
      const answers = native(texts);
      for (const [index, text] of texts.entries()) {
        const expected = answers[index];
        if (expected === TIMEOUT) {
          skipped++;
          continue;
        }
        compared++;
        if (expected) matched++;
        const actual = compiled.matches(text);
        if (actual !== expected) disagreements.push(`${JSON.stringify(source)} on ${JSON.stringify(text)}: engine ${actual}, native ${expected}`);
      }
    }
    expect(disagreements.slice(0, 10)).toEqual([]);
    expect(compared).toBeGreaterThanOrEqual(60_000);
    expect(skipped / (compared + skipped)).toBeLessThan(0.01);
    expect(matched / compared).toBeGreaterThan(0.15);
    expect(matched / compared).toBeLessThan(0.85);
  }, 120_000);
});
