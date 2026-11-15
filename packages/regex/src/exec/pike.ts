import { contains } from "../program/ranges";
import type { Inst } from "../program/types";
import { ASSERT, EDGE, JUMP, RANGES, SPLIT, flatten, holds, sideOf, type Executor } from "./flat";

const LAST_GENERATION = 2 ** 31 - 2;

export function createPike(program: readonly Inst[]): Executor {
  const length = program.length;
  const { ops, first, second, sets } = flatten(program);

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
