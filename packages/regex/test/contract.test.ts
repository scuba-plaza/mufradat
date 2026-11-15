import { describe, expect, it } from "vitest";
import { compile } from "../src";
import { DEFAULT_LIMITS, parse } from "../src/syntax/parse";
import type { Limits, RegexError } from "../src/syntax/types";
import { AREAS } from "./contract";
import { build } from "./support/pipeline";
import { referenceMatcher } from "./support/reference";

interface Subject {
  name: string;
  answer(pattern: string, text: string): boolean | RegexError;
  refusal(pattern: string, limits: Limits): RegexError | null;
}

const reference: Subject = {
  name: "the parser and the reference matcher",
  answer(pattern, text) {
    const parsed = parse(pattern);
    return "error" in parsed ? parsed.error : referenceMatcher(parsed.tree)(text);
  },
  refusal(pattern, limits) {
    const built = build(pattern, limits);
    return "error" in built ? built.error : null;
  },
};

const engine: Subject = {
  name: "compile()",
  answer(pattern, text) {
    const compiled = compile(pattern);
    return "code" in compiled ? compiled : compiled.test(text);
  },
  refusal(pattern, limits) {
    const compiled = compile(pattern, limits);
    return "code" in compiled ? compiled : null;
  },
};

const show = (value: unknown) => JSON.stringify(value);

for (const subject of [reference, engine]) {
  describe.each(AREAS)(`the contract through ${subject.name}: $name`, (area) => {
    it("answers every row as the contract says", () => {
      const failures: string[] = [];
      for (const row of area.matches) {
        const actual = subject.answer(row.pattern, row.text);
        if (typeof actual !== "boolean") failures.push(`${show(row.pattern)} was refused with ${actual.code}`);
        else if (actual !== row.matches) failures.push(`${show(row.pattern)} on ${show(row.text)}: expected ${row.matches}, got ${actual}`);
      }
      expect(failures).toEqual([]);
    });

    it("refuses every refused pattern with its code, place and detail", () => {
      const failures: string[] = [];
      for (const row of area.refused) {
        const error = subject.refusal(row.pattern, { ...DEFAULT_LIMITS, ...row.limits });
        const expected = { code: row.code, position: row.position, detail: row.detail };
        if (!error) failures.push(`${show(row.pattern)} was accepted, expected ${show(expected)}`);
        else if (show({ code: error.code, position: error.position, detail: error.detail }) !== show(expected)) {
          failures.push(`${show(row.pattern)}: expected ${show(expected)}, got ${show(error)}`);
        }
      }
      expect(failures).toEqual([]);
    });
  });
}
