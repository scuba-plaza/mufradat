import { describe, expect, it } from "vitest";
import { ASSERT, EDGE, OTHER, PENDING, WORD, createClosure, flatten } from "../src/exec/flat";
import { build } from "./support/pipeline";
import { seeded } from "./support/random";

const flatOf = (pattern: string) => {
  const built = build(pattern);
  if ("error" in built) throw new Error(`${pattern} was refused with ${built.error.code}`);
  return { flat: flatten(built.program), length: built.program.length };
};

const closureOf = (pattern: string, before: number, after: number, lastGeneration?: number) => {
  const { flat, length } = flatOf(pattern);
  const closure = createClosure(flat, length, lastGeneration);
  const list = new Int32Array(length);
  return () => {
    closure.begin();
    const count = closure.add(list, 0, 0, before, after);
    return count < 0 ? count : Array.from(list.slice(0, count));
  };
};

describe("the closure of a program position", () => {
  it("finds the same positions after its generation counter has wrapped, so that a mark left by an earlier round does not hide one", () => {
    const sides = [EDGE, WORD, OTHER];
    for (const pattern of ["^a|b|\\bc", "(^a|x)*\\bb", "(\\Ba|b*)+\\b", "($|a)(\\b|c)"]) {
      const fresh = sides.map((before) => sides.map((after) => closureOf(pattern, before, after)()));
      const { flat, length } = flatOf(pattern);
      const closure = createClosure(flat, length, 4);
      const list = new Int32Array(length);
      const random = seeded(7);
      for (let round = 0; round < 500; round++) {
        const [before, after] = [random.below(sides.length), random.below(sides.length)];
        closure.begin();
        const count = closure.add(list, 0, 0, sides[before], sides[after]);
        expect(count < 0 ? count : Array.from(list.slice(0, count)), `${pattern} in round ${round}`).toEqual(fresh[before][after]);
      }
    }
  });

  it("keeps an assertion as a position when it is not resolved, and goes through it when it holds", () => {
    const { flat } = flatOf("^a");
    expect(flat.ops[0]).toBe(ASSERT);
    expect(closureOf("^a", PENDING, PENDING)()).toEqual([0]);
    expect(closureOf("^a", EDGE, OTHER)()).toEqual([1]);
    expect(closureOf("^a", OTHER, OTHER)()).toEqual([]);
  });

  it("reports a match as -1", () => {
    expect(closureOf("a*", EDGE, OTHER)()).toBe(-1);
    expect(closureOf("a*", PENDING, PENDING)()).toBe(-1);
  });
});
