import { runInNewContext } from "node:vm";
import { describe, expect, it } from "vitest";
import { openStore } from "./store";
import { view } from "./words";

const BUDGET_MS = 10_000;
const TEXT_LENGTH = 1_000;
const TAG_LENGTH = 40;
const TOO_SLOW = Symbol("too slow");

const KILLERS = [
  "^(a+)+$",
  "(a*)*b",
  "(a|aa)+c",
  "^(a|a)*$",
  "^([a-z]+)*$",
  "^(\\w+\\s?)*$",
  "(.*a){12,12}b",
  "(a?){30,30}a{30,30}b",
];

const FIELDS = ["re:", "root:re:", "tag:re:"];

const nested = (depth: number) => "(".repeat(depth) + "a" + "){2,2}".repeat(depth);
const BOMBS = [nested(27), "((a{100,100}){100,100}){100,100}"];

const timedOut = (error: unknown) => typeof error === "object" && error !== null && "code" in error && error.code === "ERR_SCRIPT_EXECUTION_TIMEOUT";

function within<T>(ms: number, run: () => T): T | typeof TOO_SLOW {
  const box: { value?: T } = {};
  try {
    runInNewContext("run()", { run: () => { box.value = run(); } }, { timeout: ms });
  } catch (error) {
    if (timedOut(error)) return TOO_SLOW;
    throw error;
  }
  return box.value as T;
}

describe("the time a re: search takes, from the search bar to the matched rows", () => {
  it("stays polynomial in the text and in the pattern for everything the pattern language can say", () => {
    const store = openStore(":memory:");
    const now = new Date();
    const long = "a".repeat(TEXT_LENGTH) + "!";
    const card = store.createNote({ lemma: "!", english: long, root: long, pos: "noun" });
    store.setTags(card.id, ["a".repeat(TAG_LENGTH - 1) + "!"]);

    const search = (query: string) => {
      const outcome = within(BUDGET_MS, () => view(store, query, now));
      if (outcome === TOO_SLOW) expect.fail(`"${query}" did not finish within ${BUDGET_MS} ms: a stage of the search is no longer polynomial`);
      return outcome;
    };
    const finds = (query: string) => search(query).rows.some((row) => row.id === card.id);

    for (const killer of KILLERS) {
      for (const field of FIELDS) {
        const found = search(field + killer);
        expect(found.error, field + killer).toBeNull();
        expect(found.rows.some((row) => row.id === card.id), field + killer).toBe(false);
      }
    }

    for (const field of FIELDS) expect(finds(`${field}^a+!$`), `${field}^a+!$`).toBe(true);

    for (const bomb of BOMBS) {
      const refused = search(`re:${bomb}`);
      expect(refused.error, bomb).toContain("too big");
      expect(refused.rows).toEqual([]);
    }

    expect(finds(Array(12).fill("root:re:(a|aa)+!$").join(" "))).toBe(true);
  }, 120_000);
});
