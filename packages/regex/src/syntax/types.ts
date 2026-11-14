export type ClassItem = { kind: "range"; from: number; to: number };

export type Node =
  | { kind: "literal"; code: number }
  | { kind: "class"; negated: boolean; items: ClassItem[] }
  | { kind: "concat"; items: Node[] }
  | { kind: "alternate"; branches: Node[] };

export type ErrorCode =
  | "too_long"
  | "nesting_depth"
  | "missing_paren"
  | "unexpected_paren"
  | "missing_bracket"
  | "empty_class"
  | "reversed_char_range";

export interface RegexError {
  code: ErrorCode;
  position?: number;
  detail?: string;
}

export interface Limits {
  maxLength: number;
  maxDepth: number;
}
