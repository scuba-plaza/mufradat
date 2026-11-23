import { contains, type Ranges } from "../program/ranges";
import type { Inst } from "../program/types";
import { PERL } from "../unicode";

export interface Executor {
  test(text: string): boolean;
}

export const RANGES = 0;
const SPLIT = 1;
const JUMP = 2;
export const ASSERT = 3;
const MATCH = 4;

export const START = 0;
const END = 1;
export const BOUNDARY = 2;
export const NOT_BOUNDARY = 3;
const ASSERTIONS = { start: START, end: END, wordBoundary: BOUNDARY, notWordBoundary: NOT_BOUNDARY };

export const EDGE = 0;
export const WORD = 1;
export const OTHER = 2;

const ASCII_SIDE = Uint8Array.from({ length: 128 }, (_, code) => (contains(PERL.w, code) ? WORD : OTHER));

export function sideOf(code: number): number {
  if (code < 0) return EDGE;
  if (code < 128) return ASCII_SIDE[code];
  return contains(PERL.w, code) ? WORD : OTHER;
}

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

const LAST_GENERATION = 2 ** 31 - 2;

export const PENDING = -1;

export interface Closure {
  begin(): void;
  claim(at: number): boolean;
  add(list: Int32Array, count: number, from: number, before: number, after: number): number;
}

export function createClosure({ ops, first, second }: Flat, length: number, lastGeneration = LAST_GENERATION): Closure {
  const visited = new Int32Array(length);
  const stack = new Int32Array(2 * length + 2);
  let generation = 0;

  const claim = (at: number): boolean => {
    if (visited[at] === generation) return false;
    visited[at] = generation;
    return true;
  };

  return {
    begin() {
      if (generation >= lastGeneration) {
        visited.fill(0);
        generation = 0;
      }
      generation++;
    },
    claim,
    add(list, count, from, before, after) {
      let top = 0;
      stack[top++] = from;
      while (top > 0) {
        const at = stack[--top];
        if (!claim(at)) continue;
        switch (ops[at]) {
          case RANGES:
            list[count++] = at;
            break;
          case SPLIT:
            stack[top++] = second[at];
            stack[top++] = first[at];
            break;
          case JUMP:
            stack[top++] = first[at];
            break;
          case ASSERT:
            if (before === PENDING) list[count++] = at;
            else if (holds(first[at], before, after)) stack[top++] = at + 1;
            break;
          default:
            return -1;
        }
      }
      return count;
    },
  };
}
