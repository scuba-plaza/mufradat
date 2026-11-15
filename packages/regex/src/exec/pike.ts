import { contains, type Ranges } from "../program/ranges";
import type { Inst } from "../program/types";
import { PERL } from "../unicode";

export interface Executor {
  test(text: string): boolean;
}

const RANGES = 0;
const SPLIT = 1;
const JUMP = 2;
const ASSERT = 3;
const MATCH = 4;

const START = 0;
const END = 1;
const BOUNDARY = 2;
const NOT_BOUNDARY = 3;
const ASSERTIONS = { start: START, end: END, wordBoundary: BOUNDARY, notWordBoundary: NOT_BOUNDARY };

const EDGE = 0;
const WORD = 1;
const OTHER = 2;

const LAST_GENERATION = 2 ** 31 - 2;

const ASCII_SIDE = Uint8Array.from({ length: 128 }, (_, code) => (contains(PERL.w, code) ? WORD : OTHER));

const sideOf = (code: number): number => (code < 0 ? EDGE : code < 128 ? ASCII_SIDE[code] : contains(PERL.w, code) ? WORD : OTHER);

function holds(assertion: number, before: number, after: number): boolean {
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

export function createPike(program: readonly Inst[]): Executor {
  const length = program.length;
  const ops = new Uint8Array(length);
  const first = new Int32Array(length);
  const second = new Int32Array(length);
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

  let current = new Int32Array(length);
  let next = new Int32Array(length);
  const visited = new Int32Array(length);
  const stack = new Int32Array(2 * length + 2);
  let generation = 0;

  const add = (list: Int32Array, count: number, start: number, before: number, after: number): number => {
    let top = 0;
    stack[top++] = start;
    while (top > 0) {
      const at = stack[--top];
      if (visited[at] === generation) continue;
      visited[at] = generation;
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
          if (holds(first[at], before, after)) stack[top++] = at + 1;
          break;
        default:
          return -1;
      }
    }
    return count;
  };

  return {
    test(text) {
      const end = text.length;
      if (generation + end + 2 > LAST_GENERATION) {
        visited.fill(0);
        generation = 0;
      }
      generation++;
      let count = 0;
      let index = 0;
      let code = end === 0 ? -1 : text.codePointAt(0)!;
      let before = EDGE;
      let after = sideOf(code);
      for (;;) {
        count = add(current, count, 0, before, after);
        if (count < 0) return true;
        if (code < 0) return false;
        const nextIndex = index + (code > 0xffff ? 2 : 1);
        const nextCode = nextIndex < end ? text.codePointAt(nextIndex)! : -1;
        const nextAfter = sideOf(nextCode);
        generation++;
        let nextCount = 0;
        for (let thread = 0; thread < count; thread++) {
          const at = current[thread];
          if (contains(sets[first[at]], code)) {
            nextCount = add(next, nextCount, at + 1, after, nextAfter);
            if (nextCount < 0) return true;
          }
        }
        const done = current;
        current = next;
        next = done;
        count = nextCount;
        index = nextIndex;
        code = nextCode;
        before = after;
        after = nextAfter;
      }
    },
  };
}
