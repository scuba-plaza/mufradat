import { contains, type Ranges } from "../program/ranges";
import type { Inst } from "../program/types";
import { PERL } from "../unicode";

export interface Executor {
  test(text: string): boolean;
}

export const RANGES = 0;
export const SPLIT = 1;
export const JUMP = 2;
export const ASSERT = 3;
export const MATCH = 4;

export const START = 0;
export const END = 1;
export const BOUNDARY = 2;
export const NOT_BOUNDARY = 3;
const ASSERTIONS = { start: START, end: END, wordBoundary: BOUNDARY, notWordBoundary: NOT_BOUNDARY };

export const EDGE = 0;
export const WORD = 1;
export const OTHER = 2;

const ASCII_SIDE = Uint8Array.from({ length: 128 }, (_, code) => (contains(PERL.w, code) ? WORD : OTHER));

export const sideOf = (code: number): number => (code < 0 ? EDGE : code < 128 ? ASCII_SIDE[code] : contains(PERL.w, code) ? WORD : OTHER);

export function holds(assertion: number, before: number, after: number): boolean {
  switch (assertion) {
    case START:
      return before === EDGE;
    case END:
      return after === EDGE;
    case BOUNDARY:
      return (before === WORD) !== (after === WORD);
    default:
      return (before === WORD) === (after === WORD);
  }
}

export interface Flat {
  ops: Uint8Array;
  first: Int32Array;
  second: Int32Array;
  sets: Ranges[];
}

export function flatten(program: readonly Inst[]): Flat {
  const ops = new Uint8Array(program.length);
  const first = new Int32Array(program.length);
  const second = new Int32Array(program.length);
  const sets: Ranges[] = [];
  const setIds = new Map<Ranges, number>();
  program.forEach((inst, at) => {
    switch (inst.op) {
      case "ranges": {
        let id = setIds.get(inst.ranges);
        if (id === undefined) {
          id = sets.length;
          sets.push(inst.ranges);
          setIds.set(inst.ranges, id);
        }
        ops[at] = RANGES;
        first[at] = id;
        return;
      }
      case "split":
        ops[at] = SPLIT;
        first[at] = inst.first;
        second[at] = inst.second;
        return;
      case "jump":
        ops[at] = JUMP;
        first[at] = inst.target;
        return;
      case "assert":
        ops[at] = ASSERT;
        first[at] = ASSERTIONS[inst.assertion];
        return;
      case "match":
        ops[at] = MATCH;
        return;
    }
  });
  return { ops, first, second, sets };
}
