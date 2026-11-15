import { describe, expect, it } from "vitest";
import { MAX_CODE_POINT, negate } from "../src/program/ranges";
import { size } from "../src/program/size";
import type { Inst } from "../src/program/types";
import { DEFAULT_LIMITS } from "../src/syntax/parse";
import { PROPERTIES } from "../src/unicode/tables";
import { generate, renderEngine } from "./support/generator";
import { build } from "./support/pipeline";
import { seeded } from "./support/random";

const program = (pattern: string): Inst[] => {
  const built = build(pattern);
  if ("error" in built) throw new Error(`${pattern} was refused with ${built.error.code}`);
  return built.program;
};

const rangesOf = (inst: Inst): readonly number[] => {
  if (inst.op !== "ranges") throw new Error(`${inst.op} is not a set`);
  return inst.ranges;
};

const lit = (code: number): Inst => ({ op: "ranges", ranges: [code, code] });
const split = (first: number, second: number): Inst => ({ op: "split", first, second });
const jump = (target: number): Inst => ({ op: "jump", target });
const MATCH: Inst = { op: "match" };

describe("the program of one construct", () => {
  it("is a match alone for the empty pattern, and one instruction per literal", () => {
    expect(program("")).toEqual([MATCH]);
    expect(program("ab")).toEqual([lit(97), lit(98), MATCH]);
  });

  it("splits an alternation into the branches, each leaving by a jump to the end", () => {
    expect(program("a|b")).toEqual([split(1, 3), lit(97), jump(4), lit(98), MATCH]);
    expect(program("a|b|c")).toEqual([split(1, 3), lit(97), jump(7), split(4, 6), lit(98), jump(7), lit(99), MATCH]);
  });

  it("loops back for a star and a plus, and skips for a question mark", () => {
    expect(program("a*")).toEqual([split(1, 3), lit(97), jump(0), MATCH]);
    expect(program("a+")).toEqual([lit(97), split(0, 2), MATCH]);
    expect(program("a?")).toEqual([split(1, 2), lit(97), MATCH]);
  });

  it("emits no copy of a body repeated zero times, and one copy for exactly one", () => {
    expect(program("a{0}")).toEqual([MATCH]);
    expect(program("a{1}")).toEqual([lit(97), MATCH]);
  });

  it("holds a class, the dot and the perl classes as sorted ranges", () => {
    expect(program("[a-c]")).toEqual([{ op: "ranges", ranges: [97, 99] }, MATCH]);
    expect(program("[cab]")).toEqual([{ op: "ranges", ranges: [97, 99] }, MATCH]);
    expect(program("[^a]")).toEqual([{ op: "ranges", ranges: [0, 96, 98, MAX_CODE_POINT] }, MATCH]);
    expect(program(".")).toEqual([{ op: "ranges", ranges: [0, 9, 11, MAX_CODE_POINT] }, MATCH]);
    expect(program("\\d")).toEqual([{ op: "ranges", ranges: PROPERTIES.Nd }, MATCH]);
    expect(program("\\D")).toEqual([{ op: "ranges", ranges: negate(PROPERTIES.Nd) }, MATCH]);
    expect(program("[\\d\\D]")).toEqual([{ op: "ranges", ranges: [0, MAX_CODE_POINT] }, MATCH]);
    expect(program("\\p{Arabic}")).toEqual([{ op: "ranges", ranges: PROPERTIES.Arabic }, MATCH]);
    expect(program("\\P{Arabic}")).toEqual([{ op: "ranges", ranges: negate(PROPERTIES.Arabic) }, MATCH]);
  });

  it("makes \\w the letters, the numbers and the underscore, and \\s the white space", () => {
    const word = program("\\w")[0];
    if (word.op !== "ranges") throw new Error("not a set");
    const holds = (code: number) => word.ranges.some((value, index) => index % 2 === 0 && code >= value && code <= word.ranges[index + 1]);
    expect([0x61, 0x5f, 0x30, 0x643, 0x665, 0xb2].map(holds)).toEqual([true, true, true, true, true, true]);
    expect([0x2d, 0x20, 0x64e].map(holds)).toEqual([false, false, false]);
    expect(program("\\s")[0]).toEqual({ op: "ranges", ranges: [9, 13, 32, 32, 160, 160, 5760, 5760, 8192, 8202, 8232, 8233, 8239, 8239, 8287, 8287, 12288, 12288, 65279, 65279] });
  });

  it("keeps an assertion as one instruction", () => {
    expect(program("^a$")).toEqual([{ op: "assert", assertion: "start" }, lit(97), { op: "assert", assertion: "end" }, MATCH]);
    expect(program("\\b\\B")).toEqual([{ op: "assert", assertion: "wordBoundary" }, { op: "assert", assertion: "notWordBoundary" }, MATCH]);
  });
});

describe("the copies of a repeated body", () => {
  it("share one set of ranges, so that a repeat does not compute and hold a set for every copy", () => {
    const copies = program(".{3}").slice(0, 3);
    expect(copies[1]).not.toBe(copies[0]);
    expect(rangesOf(copies[1])).toBe(rangesOf(copies[0]));
    expect(rangesOf(copies[2])).toBe(rangesOf(copies[0]));
  });
});

describe("a bounded repeat", () => {
  it("is built as the nested form, so that every optional copy leaves by the same exit", () => {
    expect(program("a{2,5}")).toEqual([lit(97), lit(97), split(3, 8), lit(97), split(5, 8), lit(97), split(7, 8), lit(97), MATCH]);
    expect(program("a{0,3}")).toEqual([split(1, 6), lit(97), split(3, 6), lit(97), split(5, 6), lit(97), MATCH]);
  });

  it("is a run of copies and a plus when it has no upper bound", () => {
    expect(program("a{3,}")).toEqual([lit(97), lit(97), lit(97), split(2, 4), MATCH]);
    expect(program("a{1,}")).toEqual([lit(97), split(0, 2), MATCH]);
    expect(program("a{0,}")).toEqual(program("a*"));
  });

  it("nests inside a group that is itself repeated", () => {
    expect(program("(ab){0,2}")).toEqual([split(1, 6), lit(97), lit(98), split(4, 6), lit(97), lit(98), MATCH]);
  });
});

describe("the program of (.{100,100}){0,100}\\u0001", () => {
  const compiled = program("(.{100,100}){0,100}\\u0001");

  it("has the size the tree gives, a hundred copies of a hundred dots with a split before each, then the literal and a match", () => {
    expect(compiled.length).toBe(100 * (100 + 1) + 1 + 1);
    expect(compiled[compiled.length - 1]).toEqual(MATCH);
    expect(compiled[compiled.length - 2]).toEqual(lit(1));
  });

  it("is nested: each of the hundred splits goes into its copy or to the one exit after all of them", () => {
    const exit = 100 * 101;
    const splits = compiled.flatMap((inst, at) => (inst.op === "split" ? [{ at, inst }] : []));
    expect(splits.length).toBe(100);
    expect(splits.map(({ at, inst }) => [at, inst])).toEqual(Array.from({ length: 100 }, (_, level) => [level * 101, split(level * 101 + 1, exit)]));
    expect(compiled[exit]).toEqual(lit(1));
  });
});

describe("the size of a tree, found before anything is built", () => {
  it("is the length of the program less its match, for random patterns of all of the syntax", () => {
    const random = seeded(7);
    const roomy = { ...DEFAULT_LIMITS, maxLength: 1_000, maxInstructions: 10_000_000 };
    const wrong: string[] = [];
    for (let k = 0; k < 1_500; k++) {
      const pattern = renderEngine(generate(random, "all", 200));
      const built = build(pattern, roomy);
      if ("error" in built) wrong.push(`${pattern} was refused with ${built.error.code}`);
      else if (size(built.tree) !== built.program.length - 1) wrong.push(`${pattern}: size ${size(built.tree)}, program ${built.program.length - 1}`);
    }
    expect(wrong).toEqual([]);
  });
});

describe("the limit on the size of a program", () => {
  const pieces = (last: number) => `(a{100,100}){100,100}(a{100,100}){99,99}a{${last},${last}}`;

  it("allows exactly 20,000 instructions, the match included, and refuses one more", () => {
    const allowed = build(pieces(99));
    expect("error" in allowed ? allowed.error : allowed.program.length).toBe(20_000);
    expect(build(pieces(100))).toEqual({ error: { code: "too_big" } });
  });

  it("refuses a pattern whose size would explode, without building it", () => {
    for (const bomb of ["((a{100,100}){100,100}){100,100}", "(((a{100,100}){100,100}){100,100}){100,100}", "(a{100,100}){100,100}|(a{100,100}){100,100}"]) {
      expect(build(bomb), bomb).toEqual({ error: { code: "too_big" } });
    }
  });

  it("refuses by arithmetic even when the size would overflow a number", () => {
    const levels = 200;
    const nest = "(".repeat(levels) + "a" + "){100,100}".repeat(levels);
    expect(build(nest, { ...DEFAULT_LIMITS, maxLength: 10_000 })).toEqual({ error: { code: "too_big" } });
  });
});
