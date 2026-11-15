import { MAX_CODE_POINT, contains, type Ranges } from "../program/ranges";
import type { Inst } from "../program/types";
import { PERL } from "../unicode";
import { ASSERT, BOUNDARY, EDGE, NOT_BOUNDARY, OTHER, PENDING, RANGES, START, createClosure, flatten, holds, sideOf, type Executor } from "./flat";
import { createPike } from "./pike";

export interface DfaOptions {
  budget?: number;
  bailBuilds?: number;
  bailRatio?: number;
}

export interface DfaStats {
  builds: number;
  transitions: number;
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
  accepts: number;
  chained: State | undefined;
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
  const flat = flatten(program);
  const { ops, first, sets } = flat;

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
  let basic: Int32Array | undefined;
  const basicClasses = (): Int32Array => {
    const table = new Int32Array(0x10000);
    for (let cls = 0; cls < classCount && bounds[cls] < 0x10000; cls++) {
      table.fill(cls, bounds[cls], cls + 1 < classCount ? Math.min(bounds[cls + 1], 0x10000) : 0x10000);
    }
    return table;
  };

  const closure = createClosure(flat, length);
  const expanded = new Int32Array(length);
  const scratch = new Int32Array(length);

  const expand = (state: State, after: number): number => {
    closure.begin();
    let count = 0;
    for (const at of state.pcs) {
      if (ops[at] === RANGES) {
        if (closure.claim(at)) expanded[count++] = at;
      } else if (holds(first[at], state.before, after)) {
        count = closure.add(expanded, count, at + 1, state.before, after);
        if (count < 0) return -1;
      }
    }
    return count;
  };

  const stats: DfaStats = { builds: 0, transitions: 0, flushes: 0, bailed: false, scanned: 0 };
  const freshTable = () => new Int32Array(16 * classCount).fill(UNKNOWN);
  let table = freshTable();
  let states: State[] = [];
  let cache = new Map<number, State>();
  let stored = 0;
  let start: State | undefined;

  const hashOf = (before: number, pcs: Int32Array): number => {
    let hash = Math.imul(2166136261 ^ before, 16777619);
    for (let k = 0; k < pcs.length; k++) hash = Math.imul(hash ^ pcs[k], 16777619);
    return hash;
  };

  const sameState = (state: State, before: number, pcs: Int32Array): boolean => {
    if (state.before !== before || state.pcs.length !== pcs.length) return false;
    for (let k = 0; k < pcs.length; k++) if (state.pcs[k] !== pcs[k]) return false;
    return true;
  };

  const intern = (before: number, sorted: Int32Array): State => {
    const hash = hashOf(before, sorted);
    const head = cache.get(hash);
    for (let known = head; known; known = known.chained) if (sameState(known, before, sorted)) return known;
    const pcs = sorted.slice();
    const created: State = { id: states.length, before, pcs, accepts: -1, chained: head };
    stored += pcs.length + classCount;
    stats.builds++;
    if (stored > budget) {
      stats.flushes++;
      created.id = 0;
      created.chained = undefined;
      states = [created];
      cache = new Map([[hash, created]]);
      stored = pcs.length + classCount;
      table = freshTable();
      start = undefined;
      return created;
    }
    states.push(created);
    cache.set(hash, created);
    if ((created.id + 1) * classCount > table.length) {
      const grown = new Int32Array(2 * table.length).fill(UNKNOWN);
      grown.set(table);
      table = grown;
    }
    return created;
  };

  closure.begin();
  const startSize = closure.add(scratch, 0, 0, PENDING, PENDING);
  const startPcs = startSize < 0 ? null : scratch.slice(0, startSize).sort();

  const build = (from: State, cls: number): number => {
    stats.transitions++;
    const row = from.id * classCount + cls;
    const code = bounds[cls];
    const after = sideOf(code);
    const count = expand(from, after);
    if (count < 0) return (table[row] = MATCHED);
    closure.begin();
    let size = 0;
    for (let k = 0; k < count; k++) {
      const at = expanded[k];
      if (contains(sets[first[at]], code)) {
        size = closure.add(scratch, size, at + 1, PENDING, PENDING);
        if (size < 0) return (table[row] = MATCHED);
      }
    }
    size = closure.add(scratch, size, 0, PENDING, PENDING);
    if (size < 0) return (table[row] = MATCHED);
    const flushes = stats.flushes;
    const target = intern(afterChar(after), scratch.subarray(0, size).sort());
    if (stats.builds > bailBuilds && stats.builds * bailRatio > stats.scanned) {
      stats.bailed = true;
      return BAILED;
    }
    const offset = target.id * classCount;
    if (flushes === stats.flushes) table[row] = offset;
    return offset;
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
      const classes = basic ?? (basic = basicClasses());
      let offset = (start ?? (start = intern(EDGE, startPcs))).id * classCount;
      let transitions = table;
      const end = text.length;
      let index = 0;
      let counted = 0;
      while (index < end) {
        const code = text.codePointAt(index)!;
        index += code > 0xffff ? 2 : 1;
        const cls = code < 0x10000 ? classes[code] : classOf(code);
        let target = transitions[offset + cls];
        if (target < 0) {
          if (target === UNKNOWN) {
            stats.scanned += index - counted;
            counted = index;
            target = build(states[offset / classCount], cls);
            transitions = table;
            if (target === BAILED) return fallback().test(text);
          }
          if (target === MATCHED) {
            stats.scanned += index - counted;
            return true;
          }
        }
        offset = target;
      }
      stats.scanned += end - counted;
      return accepts(states[offset / classCount]);
    },
  };
}
