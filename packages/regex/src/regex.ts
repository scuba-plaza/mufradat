import { createDfa } from "./exec/dfa";
import { compileProgram } from "./program/compile";
import { DEFAULT_LIMITS, parse } from "./syntax/parse";
import type { Limits, RegexError } from "./syntax/types";

export interface Matcher {
  test(text: string): boolean;
}

const LIMIT_NAMES = Object.keys(DEFAULT_LIMITS) as (keyof Limits)[];

function effectiveLimits(limits: Partial<Limits>): Limits {
  const effective = { ...DEFAULT_LIMITS };
  for (const name of LIMIT_NAMES) {
    const value = limits[name];
    if (value !== undefined) effective[name] = value;
  }
  return effective;
}

function build(pattern: string, limits: Limits): Matcher | RegexError {
  const parsed = parse(pattern, limits);
  if ("error" in parsed) return parsed.error;
  const compiled = compileProgram(parsed.tree, limits);
  if ("error" in compiled) return compiled.error;
  const executor = createDfa(compiled.program);
  return { test: (text) => executor.test(text) };
}

export function compile(pattern: string, limits: Partial<Limits> = {}): Matcher | RegexError {
  try {
    return build(pattern, effectiveLimits(limits));
  } catch (failure) {
    if (failure instanceof RangeError) return { code: "nesting_depth" };
    throw failure;
  }
}
