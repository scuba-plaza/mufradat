import { describe, expect, it } from "vitest";
import { compile } from "../src";

describe("compile() and its limits", () => {
  it("keeps a default limit that is given as undefined", () => {
    expect(compile("a{101}", { maxRepeat: undefined })).toEqual({ code: "repeat_too_large", position: 1 });
    expect(compile("a".repeat(300), { maxLength: undefined })).toEqual({ code: "too_long" });
    expect(compile("(".repeat(1_001) + ")".repeat(1_001), { maxLength: 5_000, maxDepth: undefined })).toEqual({ code: "nesting_depth", position: 1_000 });
  });

  it("takes a limit that is given", () => {
    const matcher = compile("a{101}", { maxRepeat: 101 });
    if ("code" in matcher) throw new Error(matcher.code);
    expect(matcher.test("a".repeat(101))).toBe(true);
    expect(matcher.test("a".repeat(100))).toBe(false);
    expect(compile("abc", { maxLength: 2 })).toEqual({ code: "too_long" });
  });

  it("refuses a pattern nested deeper than the stack can hold instead of throwing, whatever the limits say", () => {
    const depth = 20_000;
    const pattern = "(".repeat(depth) + "a" + ")".repeat(depth);
    expect(compile(pattern, { maxLength: 100_000, maxDepth: 1e9, maxInstructions: 1e9 })).toEqual({ code: "nesting_depth" });
  });
});
