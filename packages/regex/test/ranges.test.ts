import { describe, expect, it } from "vitest";
import { MAX_CODE_POINT, contains, negate, normalize, union } from "../src/program/ranges";
import { seeded, type Random } from "./support/random";

const LOW = Array.from({ length: 41 }, (_, code) => code);
const HIGH = [MAX_CODE_POINT - 3, MAX_CODE_POINT - 2, MAX_CODE_POINT - 1, MAX_CODE_POINT];
const PROBES = [...LOW, ...HIGH];
const CASES = 400;

function randomPairs(random: Random): number[] {
  const pairs: number[] = [];
  for (let k = random.between(0, 6); k > 0; k--) {
    const pool = random.chance(0.8) ? LOW : PROBES;
    const [a, b] = [random.pick(pool), random.pick(pool)];
    pairs.push(Math.min(a, b), Math.max(a, b));
  }
  return pairs;
}

const naive = (pairs: readonly number[]): Set<number> => {
  const found = new Set<number>();
  for (let i = 0; i < pairs.length; i += 2) for (const code of PROBES) if (code >= pairs[i] && code <= pairs[i + 1]) found.add(code);
  return found;
};

function canonical(ranges: readonly number[]): boolean {
  if (ranges.length % 2 !== 0) return false;
  for (let i = 0; i < ranges.length; i += 2) {
    if (ranges[i] > ranges[i + 1] || ranges[i] < 0 || ranges[i + 1] > MAX_CODE_POINT) return false;
    if (i > 0 && ranges[i] <= ranges[i - 1] + 1) return false;
  }
  return true;
}

describe("character sets as sorted, merged ranges", () => {
  it("merges overlapping, adjacent and unordered ranges into the canonical form, keeping exactly the same members", () => {
    const random = seeded(1);
    for (let k = 0; k < CASES; k++) {
      const pairs = randomPairs(random);
      const merged = normalize(pairs);
      expect(canonical(merged), JSON.stringify(pairs)).toBe(true);
      const expected = naive(pairs);
      for (const code of PROBES) expect(contains(merged, code), `${JSON.stringify(pairs)} at ${code}`).toBe(expected.has(code));
    }
  });

  it("merges ranges that touch but do not overlap, and keeps ranges that leave a gap", () => {
    expect(normalize([5, 9, 1, 4])).toEqual([1, 9]);
    expect(normalize([1, 3, 5, 9])).toEqual([1, 3, 5, 9]);
    expect(normalize([1, 10, 3, 4])).toEqual([1, 10]);
    expect(normalize([7, 7, 7, 7])).toEqual([7, 7]);
    expect(normalize([])).toEqual([]);
  });

  it("answers a union with the members of either set", () => {
    const random = seeded(2);
    for (let k = 0; k < CASES; k++) {
      const [a, b] = [normalize(randomPairs(random)), normalize(randomPairs(random))];
      const joined = union(a, b);
      expect(canonical(joined)).toBe(true);
      for (const code of PROBES) expect(contains(joined, code), `${JSON.stringify([a, b])} at ${code}`).toBe(contains(a, code) || contains(b, code));
    }
  });

  it("answers a negation with exactly the code points the set lacks, and negating twice gives the set back", () => {
    const random = seeded(3);
    for (let k = 0; k < CASES; k++) {
      const set = normalize(randomPairs(random));
      const flipped = negate(set);
      expect(canonical(flipped)).toBe(true);
      for (const code of PROBES) expect(contains(flipped, code), `${JSON.stringify(set)} at ${code}`).toBe(!contains(set, code));
      expect(negate(flipped)).toEqual(set);
    }
  });

  it("negates the empty set to every code point and every code point to the empty set", () => {
    expect(negate([])).toEqual([0, MAX_CODE_POINT]);
    expect(negate([0, MAX_CODE_POINT])).toEqual([]);
    expect(negate([0, 0])).toEqual([1, MAX_CODE_POINT]);
    expect(negate([MAX_CODE_POINT, MAX_CODE_POINT])).toEqual([0, MAX_CODE_POINT - 1]);
  });

  it("finds a member by binary search as a scan would, on a large set", () => {
    const random = seeded(4);
    const pairs: number[] = [];
    for (let at = 0; at < 5_000; at++) pairs.push(at * 7, at * 7 + random.between(0, 4));
    const big = normalize(pairs);
    const scan = (code: number) => big.some((value, index) => index % 2 === 0 && code >= value && code <= big[index + 1]);
    for (let code = 0; code < 5_000 * 7 + 10; code += 1) if (contains(big, code) !== scan(code)) expect.fail(`code ${code}`);
  });
});
