import type { ErrorCode } from "@mufradat/regex";
import { describe, expect, it } from "vitest";
import { describeError } from "./errors";

describe("the sentence for a pattern error", () => {
  it.each<[ErrorCode, string]>([
    ["too_long", "The pattern is too long."],
    ["nesting_depth", "Groups are nested too deeply."],
    ["missing_paren", "A group in () is not closed."],
    ["unexpected_paren", "A ) has no matching (."],
    ["empty_class", "A class in [] is empty."],
    ["reversed_char_range", "A range in a class goes from the smaller to the larger character."],
    ["invalid_char_range", "A range in a class needs one character on each side of the dash."],
    ["trailing_backslash", "The pattern ends with a lone backslash."],
    ["missing_repeat_argument", "A quantifier has nothing before it."],
    ["reversed_repeat", "A quantifier goes from the smaller to the larger number."],
    ["repeat_too_large", "A quantifier repeats more than 100 times."],
  ])("keeps, word for word, the sentence for %s", (code, sentence) => {
    expect(describeError({ code })).toBe(sentence);
  });
});
