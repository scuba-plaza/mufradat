import { describe, expect, it } from "vitest";
import { createDfa } from "../src/exec/dfa";
import { createPike } from "../src/exec/pike";
import type { Inst } from "../src/program/types";
import { build } from "./support/pipeline";
import { seeded } from "./support/random";

function programOf(pattern: string): Inst[] {
  const built = build(pattern);
  if ("error" in built) throw new Error(`${pattern} was refused with ${built.error.code}`);
  return built.program;
}

function randomText(seed: number, length: number, letters: string): string {
  const random = seeded(seed);
  let text = "";
  for (let k = 0; k < length; k++) text += letters[random.below(letters.length)];
  return text;
}

describe("the lazy DFA", () => {
  it("builds each state once and keeps it for every later text", () => {
    const dfa = createDfa(programOf("a[ab]{3}c"));
    const text = randomText(1, 400, "ab");
    const first = dfa.test(text);
    const builtOnce = dfa.stats.builds;
    expect(builtOnce).toBeGreaterThan(0);
    expect(dfa.test(text)).toBe(first);
    expect(dfa.test(text)).toBe(first);
    expect(dfa.stats.builds).toBe(builtOnce);
    expect(dfa.stats.scanned).toBe(3 * text.length);
  });

  it("computes each transition once and keeps it when its table grows, so that a second pass computes none", () => {
    const dfa = createDfa(programOf("a[ab]{6}c"));
    const text = randomText(7, 2_000, "ab");
    expect(dfa.test(text)).toBe(false);
    expect(dfa.stats.builds).toBeGreaterThan(64);
    const computed = dfa.stats.transitions;
    expect(computed).toBe(dfa.stats.builds * 2);
    expect(dfa.test(text)).toBe(false);
    expect(dfa.stats.transitions).toBe(computed);
  });

  it("does not bail out on an ordinary pattern over a lot of text", () => {
    const dfa = createDfa(programOf("[a-z]+ing\\b"));
    const text = randomText(2, 200_000, "abcdefghijklmnopqrstuvwxyz ");
    const pike = createPike(programOf("[a-z]+ing\\b"));
    expect(dfa.test(text)).toBe(pike.test(text));
    expect(dfa.stats.bailed).toBe(false);
    expect(dfa.stats.builds).toBeLessThan(200);
  });

  it("drops its whole cache when the budget is spent, and still answers as the Pike executor does", () => {
    const program = programOf("a[ab]{6}b");
    const dfa = createDfa(program, { budget: 300, bailBuilds: Infinity });
    const pike = createPike(program);
    for (let k = 0; k < 40; k++) {
      const text = randomText(100 + k, 50 + 13 * k, "ab");
      expect(dfa.test(text), text).toBe(pike.test(text));
    }
    expect(dfa.stats.flushes).toBeGreaterThan(0);
    expect(dfa.stats.bailed).toBe(false);
  });

  it("bails out to the Pike executor for the rest of its life when it keeps building states", () => {
    const program = programOf("a.{18,18}c");
    const dfa = createDfa(program, { bailBuilds: 30 });
    const pike = createPike(program);
    const body = randomText(3, 20_000, "ab");
    for (const text of [body, body + "a" + "b".repeat(18) + "c"]) expect(dfa.test(text)).toBe(pike.test(text));
    expect(dfa.stats.bailed).toBe(true);
    const scanned = dfa.stats.scanned;
    const later = randomText(4, 300, "ab") + "a" + "a".repeat(18) + "c";
    expect(dfa.test(later)).toBe(true);
    expect(dfa.test(later.slice(1, 100))).toBe(pike.test(later.slice(1, 100)));
    expect(dfa.stats.scanned).toBe(scanned);
  });

  it("bails out by the default rule on a hostile pattern over 200,000 characters, and answers as the Pike executor does", () => {
    const program = programOf("a.{18,18}c");
    const dfa = createDfa(program);
    const text = randomText(5, 200_000, "ab");
    expect(dfa.test(text)).toBe(createPike(program).test(text));
    expect(dfa.stats.bailed).toBe(true);
    expect(dfa.stats.builds).toBeLessThan(5_000);
  });
});
