import { compileProgram } from "../../src/program/compile";
import type { Inst } from "../../src/program/types";
import { DEFAULT_LIMITS, parse } from "../../src/syntax/parse";
import type { Limits, Node, RegexError } from "../../src/syntax/types";

export type Built = { tree: Node; program: Inst[] } | { error: RegexError };

export function build(pattern: string, limits: Limits = DEFAULT_LIMITS): Built {
  const parsed = parse(pattern, limits);
  if ("error" in parsed) return parsed;
  const compiled = compileProgram(parsed.tree, limits);
  return "error" in compiled ? compiled : { tree: parsed.tree, program: compiled.program };
}
