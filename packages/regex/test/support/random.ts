export interface Random {
  below(limit: number): number;
  between(low: number, high: number): number;
  chance(probability: number): boolean;
  pick<T>(items: readonly T[]): T;
  weighted<T>(choices: readonly (readonly [number, T])[]): T;
}

export function seeded(seed: number): Random {
  let state = seed >>> 0;
  const next = (): number => {
    state = (state + 0x6d2b79f5) >>> 0;
    let mixed = Math.imul(state ^ (state >>> 15), state | 1);
    mixed ^= mixed + Math.imul(mixed ^ (mixed >>> 7), mixed | 61);
    return ((mixed ^ (mixed >>> 14)) >>> 0) / 4294967296;
  };
  const below = (limit: number): number => Math.floor(next() * limit);
  return {
    below,
    between: (low, high) => low + below(high - low + 1),
    chance: (probability) => next() < probability,
    pick: (items) => items[below(items.length)],
    weighted: (choices) => {
      const total = choices.reduce((sum, [weight]) => sum + weight, 0);
      let left = next() * total;
      for (const [weight, value] of choices) {
        left -= weight;
        if (left < 0) return value;
      }
      return choices[choices.length - 1][1];
    },
  };
}
