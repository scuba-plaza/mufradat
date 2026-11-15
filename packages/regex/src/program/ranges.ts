export type Ranges = readonly number[];

export const MAX_CODE_POINT = 0x10ffff;

export function normalize(pairs: Ranges): number[] {
  const sorted: [number, number][] = [];
  for (let at = 0; at < pairs.length; at += 2) sorted.push([pairs[at], pairs[at + 1]]);
  sorted.sort((a, b) => a[0] - b[0]);
  const merged: number[] = [];
  for (const [low, high] of sorted) {
    const last = merged.length - 1;
    if (last >= 0 && low <= merged[last] + 1) merged[last] = Math.max(merged[last], high);
    else merged.push(low, high);
  }
  return merged;
}

export const union = (a: Ranges, b: Ranges): number[] => normalize([...a, ...b]);

export function negate(set: Ranges): number[] {
  const gaps: number[] = [];
  let next = 0;
  for (let at = 0; at < set.length; at += 2) {
    if (set[at] > next) gaps.push(next, set[at] - 1);
    next = set[at + 1] + 1;
  }
  if (next <= MAX_CODE_POINT) gaps.push(next, MAX_CODE_POINT);
  return gaps;
}

export function contains(set: Ranges, code: number): boolean {
  let low = 0;
  let high = set.length / 2 - 1;
  while (low <= high) {
    const middle = (low + high) >> 1;
    if (code < set[2 * middle]) high = middle - 1;
    else if (code > set[2 * middle + 1]) low = middle + 1;
    else return true;
  }
  return false;
}
