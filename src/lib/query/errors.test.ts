import { describe, expect, it } from "vitest";
import { describeError } from "./errors";

describe("the sentence for a pattern error", () => {
  it("keeps, word for word, the sentence of every error the earlier engine had", () => {
    expect(describeError({ code: "too_long" })).toBe("The pattern is too long.");
    expect(describeError({ code: "too_big" })).toBe("The pattern is too big to search with.");
    expect(describeError({ code: "missing_paren" })).toBe("A group in () is not closed.");
    expect(describeError({ code: "unexpected_paren" })).toBe("A ) has no matching (.");
    expect(describeError({ code: "missing_bracket" })).toBe("A class in [] is not closed.");
    expect(describeError({ code: "empty_class" })).toBe("A class in [] is empty.");
    expect(describeError({ code: "reversed_char_range" })).toBe("A range in a class goes from the smaller to the larger character.");
    expect(describeError({ code: "invalid_char_range" })).toBe("A range in a class needs one character on each side of the dash.");
    expect(describeError({ code: "trailing_backslash" })).toBe("The pattern ends with a lone backslash.");
    expect(describeError({ code: "missing_repeat_argument" })).toBe("A quantifier has nothing before it.");
    expect(describeError({ code: "reversed_repeat" })).toBe("A quantifier goes from the smaller to the larger number.");
    expect(describeError({ code: "repeat_too_large" })).toBe("A quantifier repeats more than 100 times.");
  });

  it("names the construct for an escape, a property and a construct that is not supported", () => {
    expect(describeError({ code: "invalid_escape", detail: "\\q" })).toBe("\\q is not an escape this search knows.");
    expect(describeError({ code: "invalid_property", detail: "\\p{Nope}" })).toBe("\\p{Nope} is not a Unicode property such as \\p{Arabic} or \\p{Lu}.");
    expect(describeError({ code: "unsupported", detail: "(?:" })).toBe("(?: is not supported.");
  });

  it("says what a brace, a repeat of a repeat and a deep nesting did wrong", () => {
    expect(describeError({ code: "invalid_repeat" })).toBe("A { has to start a repeat such as {2}, {2,} or {2,5}; write \\{ for a brace itself.");
    expect(describeError({ code: "nested_repeat" })).toBe("A quantifier cannot follow another quantifier.");
    expect(describeError({ code: "nesting_depth" })).toBe("Groups are nested too deeply.");
  });
});
