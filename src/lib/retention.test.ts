import { describe, expect, it } from "vitest";
import { DEFAULT_LIMITS, selectRemovals, type RetentionEntry } from "./retention";

const unbounded = { ...DEFAULT_LIMITS, budgetBytes: Infinity };

const before = (id: string, reason: "before-reset" | "before-restore", dbBytes = 10): RetentionEntry => ({
  id,
  reason,
  dbBytes,
  clips: {},
});

const daily = (id: string, dbBytes = 10, clips: Record<string, number> = {}): RetentionEntry => ({
  id,
  reason: "daily",
  dbBytes,
  clips,
});

describe("choosing which automatic snapshots to drop", () => {
  it("keeps the newest 14 daily snapshots", () => {
    const entries = Array.from({ length: 15 }, (_, i) => daily(`d${String(i + 1).padStart(2, "0")}`));
    expect(selectRemovals(entries, unbounded)).toEqual(["d01"]);
  });

  it("counts before-reset and before-restore together, apart from the daily snapshots", () => {
    const dailies = Array.from({ length: 14 }, (_, i) => daily(`d${String(i + 1).padStart(2, "0")}`));
    const befores = Array.from({ length: 15 }, (_, i) =>
      before(`b${String(i + 1).padStart(2, "0")}`, i % 2 === 0 ? "before-reset" : "before-restore"),
    );
    expect(selectRemovals([...dailies, ...befores], unbounded)).toEqual(["b01"]);
  });

  it("never removes manual or imported snapshots", () => {
    const kept: RetentionEntry[] = Array.from({ length: 30 }, (_, i) => ({
      id: `m${String(i + 1).padStart(2, "0")}`,
      reason: i % 2 === 0 ? "manual" : "imported",
      dbBytes: 10,
      clips: { a: 100 },
    }));
    expect(selectRemovals(kept, { dailyKeep: 1, beforeKeep: 1, budgetBytes: 1 })).toEqual([]);
  });

  it("counts a clip shared by two snapshots once against the budget", () => {
    const entries = [daily("s1", 10, { A: 100 }), daily("s2", 10, { A: 100 }), daily("s3", 10, { B: 50 })];
    expect(selectRemovals(entries, { ...DEFAULT_LIMITS, budgetBytes: 180 })).toEqual([]);
    expect(selectRemovals(entries, { ...DEFAULT_LIMITS, budgetBytes: 175 })).toEqual(["s1"]);
    expect(selectRemovals(entries, { ...DEFAULT_LIMITS, budgetBytes: 100 })).toEqual(["s1", "s2"]);
  });

  it("keeps the newest daily and the newest before snapshot even when they alone exceed the budget", () => {
    const entries = [daily("d1", 10), daily("d2", 500), before("r1", "before-reset", 500)];
    expect(selectRemovals(entries, { ...DEFAULT_LIMITS, budgetBytes: 100 })).toEqual(["d1"]);
  });

  it("does not count manual snapshots against the budget", () => {
    const entries: RetentionEntry[] = [
      { id: "m1", reason: "manual", dbBytes: 10, clips: { A: 1000 } },
      daily("d0", 10),
      daily("d1", 10),
    ];
    expect(selectRemovals(entries, { ...DEFAULT_LIMITS, budgetBytes: 100 })).toEqual([]);
  });
});
