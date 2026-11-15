import type { Assertion } from "../syntax/types";

export type Inst =
  | { op: "ranges"; ranges: readonly number[] }
  | { op: "split"; first: number; second: number }
  | { op: "jump"; target: number }
  | { op: "assert"; assertion: Assertion }
  | { op: "match" };
