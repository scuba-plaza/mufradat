export type ClassItem =
  | { kind: "range"; from: number; to: number }
  | { kind: "perl"; name: "d" | "w" | "s"; negated: boolean }
  | { kind: "property"; name: string; negated: boolean };

export type Assertion = "start" | "end" | "wordBoundary" | "notWordBoundary";

export type Node =
  | { kind: "literal"; code: number }
  | { kind: "class"; negated: boolean; items: ClassItem[] }
  | { kind: "assert"; assertion: Assertion }
  | { kind: "concat"; items: Node[] }
  | { kind: "alternate"; branches: Node[] }
  | { kind: "repeat"; body: Node; min: number; max: number | null };

export type ErrorCode =
  | "too_long"
  | "nesting_depth"
  | "missing_paren"
  | "unexpected_paren"
  | "missing_bracket"
  | "empty_class"
  | "reversed_char_range"
  | "invalid_char_range"
  | "invalid_property"
  | "missing_repeat_argument"
  | "invalid_repeat"
  | "reversed_repeat"
  | "repeat_too_large"
  | "nested_repeat"
  | "trailing_backslash"
  | "invalid_escape"
  | "unsupported";

export interface RegexError {
  code: ErrorCode;
  position?: number;
  detail?: string;
}

export interface Limits {
  maxLength: number;
  maxRepeat: number;
  maxDepth: number;
}
