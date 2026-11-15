import { describe, expect, it } from "vitest";
import { compile, compilePattern } from "../src";
import { TIMEOUT, within } from "./support/watchdog";

const BUDGET_MS = 10_000;
const TEXT_LENGTH = 1_000;

interface Engine {
  name: string;
  compile(pattern: string): ((text: string) => boolean) | string;
  tooBig: RegExp;
}

const ENGINES: Engine[] = [
  {
    name: "the current engine",
    compile(pattern) {
      const compiled = compilePattern(pattern);
      return typeof compiled === "string" ? compiled : (text) => compiled.matches(text);
    },
    tooBig: /too big/,
  },
  {
    name: "compile()",
    compile(pattern) {
      const compiled = compile(pattern);
      return "code" in compiled ? compiled.code : (text) => compiled.test(text);
    },
    tooBig: /^too_big$/,
  },
];

const FAMILIES: readonly (readonly [pattern: string, ending: string])[] = [
  ["^(a+)+$", ""],
  ["(a*)*b", "b"],
  ["(a|aa)+c", "c"],
  ["^(a|a)*$", ""],
  ["^([a-z]+)*$", ""],
  ["^(\\w+\\s?)*$", ""],
  ["(a?){30,30}a{30,30}b", "b"],
  ["(.*a){12,12}b", "b"],
];

const nested = (depth: number) => "(".repeat(depth) + "a" + "){2,2}".repeat(depth);
const BOMBS = ["((a{100,100}){100,100}){100,100}", nested(27)];

describe.each(ENGINES)("$name", (engine) => {
  describe("on patterns that make a backtracking engine exponential", () => {
    it.each(FAMILIES)("%s is answered in time on a text of 1,000 characters", (pattern, ending) => {
      const answers = within(BUDGET_MS, () => {
        const compiled = engine.compile(pattern);
        if (typeof compiled === "string") throw new Error(compiled);
        const body = "a".repeat(TEXT_LENGTH);
        return [compiled(body + "!"), compiled(body + ending)];
      });
      if (answers === TIMEOUT) expect.fail(`${pattern} did not finish within ${BUDGET_MS} ms: the engine is no longer linear in the text`);
      expect(answers).toEqual([false, true]);
    });
  });

  describe("on patterns whose compiled size would explode", () => {
    it.each(BOMBS)("%s is refused as too big, at once", (bomb) => {
      const refused = within(BUDGET_MS, () => engine.compile(bomb));
      if (refused === TIMEOUT) expect.fail(`${bomb} was not refused within ${BUDGET_MS} ms`);
      expect(refused).toMatch(engine.tooBig);
    });
  });
});
