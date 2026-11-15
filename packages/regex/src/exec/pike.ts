import { contains } from "../program/ranges";
import type { Inst } from "../program/types";
import { EDGE, createClosure, flatten, sideOf, type Executor } from "./flat";

export function createPike(program: readonly Inst[]): Executor {
  const length = program.length;
  const flat = flatten(program);
  const { sets, first } = flat;
  const closure = createClosure(flat, length);

  let current = new Int32Array(length);
  let next = new Int32Array(length);

  return {
    test(text) {
      const end = text.length;
      closure.begin();
      let count = 0;
      let index = 0;
      let code = end === 0 ? -1 : text.codePointAt(0)!;
      let before = EDGE;
      let after = sideOf(code);
      for (;;) {
        count = closure.add(current, count, 0, before, after);
        if (count < 0) return true;
        if (code < 0) return false;
        const nextIndex = index + (code > 0xffff ? 2 : 1);
        const nextCode = nextIndex < end ? text.codePointAt(nextIndex)! : -1;
        const nextAfter = sideOf(nextCode);
        closure.begin();
        let nextCount = 0;
        for (let thread = 0; thread < count; thread++) {
          const at = current[thread];
          if (contains(sets[first[at]], code)) {
            nextCount = closure.add(next, nextCount, at + 1, after, nextAfter);
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
