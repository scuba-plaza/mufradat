import { MAX_CODE_POINT, contains, type Ranges } from "../program/ranges";
import type { Inst } from "../program/types";
import { PERL } from "../unicode";
import { ASSERT, BOUNDARY, EDGE, JUMP, NOT_BOUNDARY, OTHER, RANGES, SPLIT, START, flatten, holds, sideOf, type Executor } from "./flat";
import { createPike } from "./pike";

export interface DfaOptions {
  budget?: number;
  bailBuilds?: number;
  bailRatio?: number;
}

export interface DfaStats {
  builds: number;
  flushes: number;
  bailed: boolean;
  scanned: number;
}

export interface DfaExecutor extends Executor {
  readonly stats: DfaStats;
}

interface State {
  id: number;
  before: number;
  pcs: Int32Array;
  next: Int32Array;
  accepts: number;
}

const UNKNOWN = -1;
const MATCHED = -2;
const BAILED = -3;

function classBoundaries(sets: readonly Ranges[], withWord: boolean): Int32Array {
  const points = new Set<number>([0]);
  const add = (ranges: Ranges) => {
    for (let at = 0; at < ranges.length; at += 2) {
      points.add(ranges[at]);
      if (ranges[at + 1] < MAX_CODE_POINT) points.add(ranges[at + 1] + 1);
    }
  };
  sets.forEach(add);
  if (withWord) add(PERL.w);
  return Int32Array.from([...points].sort((a, b) => a - b));
}

export function createDfa(program: readonly Inst[], options: DfaOptions = {}): DfaExecutor {
  const budget = options.budget ?? 2_000_000;
  const bailBuilds = options.bailBuilds ?? 2_000;
  const bailRatio = options.bailRatio ?? 8;
  const length = program.length;
  const { ops, first, second, sets } = flatten(program);

  let hasStart = false;
  let hasBoundary = false;
  for (let at = 0; at < length; at++) {
    if (ops[at] !== ASSERT) continue;
    if (first[at] === START) hasStart = true;
    else if (first[at] === BOUNDARY || first[at] === NOT_BOUNDARY) hasBoundary = true;
  }
  const afterChar = (side: number): number => (hasBoundary ? side : hasStart ? OTHER : EDGE);

  const bounds = classBoundaries(sets, hasBoundary);
  const classCount = bounds.length;
  const asciiClass = new Int32Array(128);
  const classOf = (code: number): number => {
    let low = 0;
    let high = classCount - 1;
    while (low < high) {
      const middle = (low + high + 1) >> 1;
      if (bounds[middle] <= code) low = middle;
      else high = middle - 1;
    }
    return low;
  };
  for (let code = 0; code < 128; code++) asciiClass[code] = classOf(code);
  const classFor = (code: number): number => (code < 128 ? asciiClass[code] : classOf(code));

  const visited = new Int32Array(length);
  const stack = new Int32Array(2 * length + 2);
  const expanded = new Int32Array(length);
  const scratch = new Int32Array(length);
  let generation = 0;

  const addPending = (list: Int32Array, count: number, from: number): number => {
    let top = 0;
    stack[top++] = from;
    while (top > 0) {
      const at = stack[--top];
      if (visited[at] === generation) continue;
      visited[at] = generation;
      switch (ops[at]) {
        case RANGES:
        case ASSERT:
          list[count++] = at;
          break;
        case SPLIT:
          stack[top++] = second[at];
          stack[top++] = first[at];
          break;
        case JUMP:
          stack[top++] = first[at];
          break;
        default:
          return -1;
      }
    }
    return count;
  };

  const addResolved = (list: Int32Array, count: number, from: number, before: number, after: number): number => {
    let top = 0;
    stack[top++] = from;
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

  const expand = (state: State, after: number): number => {
    generation++;
    let count = 0;
    for (const at of state.pcs) {
      if (ops[at] === RANGES) {
        if (visited[at] !== generation) {
          visited[at] = generation;
          expanded[count++] = at;
        }
      } else if (holds(first[at], state.before, after)) {
        count = addResolved(expanded, count, at + 1, state.before, after);
        if (count < 0) return -1;
      }
    }
    return count;
  };

  const stats: DfaStats = { builds: 0, flushes: 0, bailed: false, scanned: 0 };
  let states: State[] = [];
  let cache = new Map<string, State>();
  let stored = 0;

  const intern = (before: number, pcs: Int32Array): State => {
    const key = `${before}|${pcs.join(",")}`;
    const known = cache.get(key);
    if (known) return known;
    const created: State = { id: states.length, before, pcs, next: new Int32Array(classCount).fill(UNKNOWN), accepts: -1 };
    states.push(created);
    cache.set(key, created);
    stored += pcs.length + classCount;
    stats.builds++;
    if (stored > budget) {
      stats.flushes++;
      created.id = 0;
      states = [created];
      cache = new Map([[key, created]]);
      stored = pcs.length + classCount;
      start = undefined;
    }
    return created;
  };

  generation++;
  const startSize = addPending(scratch, 0, 0);
  const startPcs = startSize < 0 ? null : scratch.slice(0, startSize).sort();
  let start: State | undefined;

  const build = (state: State, cls: number): number => {
    const code = bounds[cls];
    const after = sideOf(code);
    const count = expand(state, after);
    if (count < 0) return (state.next[cls] = MATCHED);
    generation++;
    let size = 0;
    for (let k = 0; k < count; k++) {
      const at = expanded[k];
      if (contains(sets[first[at]], code)) {
        size = addPending(scratch, size, at + 1);
        if (size < 0) return (state.next[cls] = MATCHED);
      }
    }
    size = addPending(scratch, size, 0);
    if (size < 0) return (state.next[cls] = MATCHED);
    const target = intern(afterChar(after), scratch.slice(0, size).sort());
    if (stats.builds > bailBuilds && stats.builds * bailRatio > stats.scanned) {
      stats.bailed = true;
      return BAILED;
    }
    return (state.next[cls] = target.id);
  };

  const accepts = (state: State): boolean => {
    if (state.accepts < 0) state.accepts = expand(state, EDGE) < 0 ? 1 : 0;
    return state.accepts === 1;
  };

  let pike: Executor | undefined;
  const fallback = (): Executor => (pike ??= createPike(program));

  return {
    stats,
    test(text) {
      if (startPcs === null) return true;
      if (stats.bailed) return fallback().test(text);
      let state = start ?? (start = intern(EDGE, startPcs));
      const end = text.length;
      let index = 0;
      while (index < end) {
        const code = text.codePointAt(index)!;
        index += code > 0xffff ? 2 : 1;
        stats.scanned++;
        const cls = classFor(code);
        let target = state.next[cls];
        if (target === UNKNOWN) {
          target = build(state, cls);
          if (target === BAILED) return fallback().test(text);
        }
        if (target === MATCHED) return true;
        state = states[target];
      }
      return accepts(state);
    },
  };
}
