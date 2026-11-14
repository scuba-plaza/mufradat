import { describe, expect, it } from "vitest";
import { DEFAULT_LIMITS, parse } from "../src/syntax/parse";
import { AREAS } from "./contract";
import { referenceMatcher } from "./support/reference";

const show = (value: unknown) => JSON.stringify(value);

describe.each(AREAS)("the contract: $name", (area) => {
  it("answers every row as the contract says", () => {
    const failures: string[] = [];
    for (const row of area.matches) {
      const parsed = parse(row.pattern);
      if ("error" in parsed) {
        failures.push(`${show(row.pattern)} was refused with ${parsed.error.code}`);
        continue;
      }
      const actual = referenceMatcher(parsed.tree)(row.text);
      if (actual !== row.matches) failures.push(`${show(row.pattern)} on ${show(row.text)}: expected ${row.matches}, got ${actual}`);
    }
    expect(failures).toEqual([]);
  });

  it("refuses every refused pattern with its code, place and detail", () => {
    const failures: string[] = [];
    for (const row of area.refused) {
      const parsed = parse(row.pattern, { ...DEFAULT_LIMITS, ...row.limits });
      const expected = { code: row.code, position: row.position, detail: row.detail };
      if (!("error" in parsed)) failures.push(`${show(row.pattern)} was accepted, expected ${show(expected)}`);
      else if (show({ code: parsed.error.code, position: parsed.error.position, detail: parsed.error.detail }) !== show(expected)) {
        failures.push(`${show(row.pattern)}: expected ${show(expected)}, got ${show(parsed.error)}`);
      }
    }
    expect(failures).toEqual([]);
  });
});
