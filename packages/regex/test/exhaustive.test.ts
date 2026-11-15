import { describe, expect, it } from "vitest";
import { compile } from "../src";
import { createDfa } from "../src/exec/dfa";
import { createPike } from "../src/exec/pike";
import { build } from "./support/pipeline";
import { referenceMatcher } from "./support/reference";

const TOKENS = ["a", "b", ".", "|", "(", ")", "*", "+", "?", "^", "$", "\\b"];
const MAX_TOKENS = 4;
const LETTERS = ["a", "b", "-"];
const MAX_LENGTH = 3;

function* sequences(alphabet: readonly string[], longest: number, shortest: number): Generator<string> {
  let level = [""];
  for (let length = 0; length <= longest; length++) {
    if (length >= shortest) yield* level;
    level = level.flatMap((prefix) => alphabet.map((token) => prefix + token));
  }
}

describe("every pattern of up to four tokens against every text of up to three characters", () => {
  it("is answered by the Pike executor, the lazy DFA and compile() as the reference matcher answers it", () => {
    const texts = [...sequences(LETTERS, MAX_LENGTH, 0)];
    const wrong: string[] = [];
    let accepted = 0;
    let compared = 0;
    let matched = 0;
    for (const pattern of sequences(TOKENS, MAX_TOKENS, 1)) {
      const built = build(pattern);
      const compiled = compile(pattern);
      if ("error" in built || "code" in compiled) {
        const same = "error" in built && "code" in compiled && built.error.code === compiled.code && built.error.position === compiled.position;
        if (!same) wrong.push(`${pattern}: build and compile do not refuse alike`);
        continue;
      }
      accepted++;
      const reference = referenceMatcher(built.tree);
      const pike = createPike(built.program);
      const dfa = createDfa(built.program);
      for (const text of texts) {
        const expected = reference(text);
        compared++;
        if (expected) matched++;
        if (pike.test(text) !== expected) wrong.push(`Pike: ${JSON.stringify(pattern)} on ${JSON.stringify(text)}: expected ${expected}`);
        if (dfa.test(text) !== expected) wrong.push(`DFA: ${JSON.stringify(pattern)} on ${JSON.stringify(text)}: expected ${expected}`);
        if (compiled.test(text) !== expected) wrong.push(`compile(): ${JSON.stringify(pattern)} on ${JSON.stringify(text)}: expected ${expected}`);
      }
    }
    expect(wrong.slice(0, 10)).toEqual([]);
    expect(accepted).toBeGreaterThan(5_000);
    expect(compared).toBe(accepted * texts.length);
    expect(matched / compared).toBeGreaterThan(0.2);
    expect(matched / compared).toBeLessThan(0.8);
  }, 300_000);
});
