export type Node =
  | { kind: "literal"; code: number }
  | { kind: "concat"; items: Node[] }
  | { kind: "alternate"; branches: Node[] };

export type ErrorCode = "too_long" | "nesting_depth" | "missing_paren" | "unexpected_paren";

export interface RegexError {
  code: ErrorCode;
  position?: number;
  detail?: string;
}

export interface Limits {
  maxLength: number;
  maxDepth: number;
}
