import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { context } from "@/lib/server";
import { openCard } from "@/lib/words";
import { PUT } from "./route";

const put = (id: string, body: unknown) =>
  PUT(
    new Request(`http://127.0.0.1/api/notes/${id}/tags`, {
      method: "PUT",
      headers: { "content-type": "application/json" },
      body: JSON.stringify(body),
    }),
    { params: Promise.resolve({ id }) },
  );

describe("setting a card's tags through the API", () => {
  let home: string;

  beforeEach(() => {
    home = fs.mkdtempSync(path.join(os.tmpdir(), "mufradat-tags-"));
    vi.stubEnv("XDG_CONFIG_HOME", path.join(home, "config"));
    vi.stubEnv("MUFRADAT_WORKSPACE", path.join(home, "workspace"));
  });
  afterEach(() => {
    vi.unstubAllEnvs();
    fs.rmSync(home, { recursive: true, force: true });
  });

  it("saves the tags as stored", async () => {
    const { store } = context();
    const id = openCard(store, { rank: 10 })!;
    const res = await put(id, { tags: ["Lesson 1", "lesson_2", "  Grammar  ", "grammar"] });
    expect(res.status).toBe(200);
    const { note } = await res.json();
    expect(note.tags).toEqual(["grammar", "lesson_1", "lesson_2"]);
    expect(store.getNote(id)!.tags).toEqual(["grammar", "lesson_1", "lesson_2"]);
  });
  it("turns every kind of space into _, including the next-line character", async () => {
    const { store } = context();
    const id = openCard(store, { rank: 10 })!;
    const res = await put(id, { tags: ["a\u0085b", "c\u00a0d", "e\tf"] });
    expect((await res.json()).note.tags).toEqual(["a_b", "c_d", "e_f"]);
  });

  it("leaves an approved card approved", async () => {
    const { store } = context();
    const id = openCard(store, { rank: 10 })!;
    store.updateNote(id, { example: "هٰذَا [كِتَابٌ] جَمِيلٌ", exampleEn: "this is a nice book" });
    store.approve(id);
    const { note } = await (await put(id, { tags: ["lesson_1"] })).json();
    expect(note.status).toBe("approved");
    expect(store.counts().approved).toBe(1);
  });

  const refusal = async (body: unknown) => {
    const { store } = context();
    const id = openCard(store, { rank: 10 })!;
    store.setTags(id, ["kept"]);
    const res = await put(id, body);
    expect(store.getNote(id)!.tags).toEqual(["kept"]);
    return { status: res.status, body: await res.json() };
  };

  it.each([["a,b"], ['a"b'], ["a*"]])("refuses %s, naming commas, quotes and *", async (tag) => {
    expect(await refusal({ tags: [tag] })).toEqual({
      status: 400,
      body: { error: "A tag cannot contain commas, quotes or *." },
    });
  });

  it("refuses a tag of more than 40 characters and takes one of exactly 40", async () => {
    expect(await refusal({ tags: ["x".repeat(41)] })).toEqual({
      status: 400,
      body: { error: "A tag is at most 40 characters." },
    });
    const id = openCard(context().store, { rank: 11 })!;
    expect((await put(id, { tags: ["x".repeat(40)] })).status).toBe(200);
  });

  it("refuses a 21st tag and takes 20", async () => {
    const many = (n: number) => Array.from({ length: n }, (_, i) => `tag${i}`);
    expect(await refusal({ tags: many(21) })).toEqual({
      status: 400,
      body: { error: "A card has at most 20 tags." },
    });
    const id = openCard(context().store, { rank: 11 })!;
    expect((await put(id, { tags: many(20) })).status).toBe(200);
  });

  it.each([[{ tags: "a" }], [{}], [{ tags: [1] }]])("refuses %j as not a list of text", async (body) => {
    expect(await refusal(body)).toEqual({ status: 400, body: { error: "tags must be a list of text." } });
  });

  it("clears the tags for an empty list", async () => {
    const { store } = context();
    const id = openCard(store, { rank: 10 })!;
    store.setTags(id, ["a"]);
    const res = await put(id, { tags: [] });
    expect(res.status).toBe(200);
    expect(store.getNote(id)!.tags).toEqual([]);
  });
});
