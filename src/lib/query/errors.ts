import type { ErrorCode, RegexError } from "@mufradat/regex";

const SENTENCES: Record<ErrorCode, string> = {
  too_long: "The pattern is too long.",
  too_big: "The pattern is too big to search with.",
  nesting_depth: "Groups are nested too deeply.",
  missing_paren: "A group in () is not closed.",
  unexpected_paren: "A ) has no matching (.",
  missing_bracket: "A class in [] is not closed.",
  empty_class: "A class in [] is empty.",
  reversed_char_range: "A range in a class goes from the smaller to the larger character.",
  invalid_char_range: "A range in a class needs one character on each side of the dash.",
  invalid_property: "{detail} is not a Unicode property such as \\p{Arabic} or \\p{Lu}.",
  trailing_backslash: "The pattern ends with a lone backslash.",
  invalid_escape: "{detail} is not an escape this search knows.",
  missing_repeat_argument: "A quantifier has nothing before it.",
  invalid_repeat: "A { has to start a repeat such as {2}, {2,} or {2,5}; write \\{ for a brace itself.",
  reversed_repeat: "A quantifier goes from the smaller to the larger number.",
  repeat_too_large: "A quantifier repeats more than 100 times.",
  nested_repeat: "A quantifier cannot follow another quantifier.",
  unsupported: "{detail} is not supported.",
};

export const describeError = ({ code, detail }: RegexError): string => SENTENCES[code].replace("{detail}", () => detail ?? "");
