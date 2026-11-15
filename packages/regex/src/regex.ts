import { createPike } from "./exec/pike";
import { compileProgram } from "./program/compile";
import { DEFAULT_LIMITS, parse } from "./syntax/parse";
import type { Limits, RegexError } from "./syntax/types";

export interface Matcher {
  test(text: string): boolean;
  testMany(texts: readonly string[]): boolean[];
}

export function compile(pattern: string, limits: Partial<Limits> = {}): Matcher | RegexError {
  const effective = { ...DEFAULT_LIMITS, ...limits };
  const parsed = parse(pattern, effective);
  if ("error" in parsed) return parsed.error;
  const compiled = compileProgram(parsed.tree, effective);
  if ("error" in compiled) return compiled.error;
  const executor = createPike(compiled.program);
  return {
    test: (text) => executor.test(text),
    testMany: (texts) => texts.map((text) => executor.test(text)),
  };
}
