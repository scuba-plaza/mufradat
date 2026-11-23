import { describe, expect, it } from "vitest";
import { context } from "@/lib/server";
import { listSnapshots } from "@/lib/snapshots";
import { openCard, wordRows } from "@/lib/words";
import { tempWorkspace, withParams } from "@/lib/testing/workspace";
import { DELETE, PUT } from "./route";

const remove = (id: string) => DELETE(new Request(`http://127.0.0.1/api/notes/${id}`, { method: "DELETE" }), withParams({ id }));

const edit = (id: string, body: Record<string, unknown>) =>
  PUT(
    new Request(`http://127.0.0.1/api/notes/${id}`, { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) }),
    withParams({ id }),
  );

describe("a card through the API", () => {
  tempWorkspace();

  it("deletes the card, gives the word back as not started, and takes no snapshot of its own", async () => {
    const { store } = context();
    const id = openCard(store, { rank: 10 })!;
    const res = await remove(id);
    expect(res.status).toBe(204);
    expect(store.getNote(id)).toBeNull();
    expect(wordRows(store)[9]).toMatchObject({ rank: 10, id: null, status: "todo" });
    expect(listSnapshots(store.root).filter((s) => s.reason !== "daily")).toEqual([]);
  });

  it("answers the same when the card is already gone", async () => {
    const id = openCard(context().store, { rank: 10 })!;
    await remove(id);
    expect((await remove(id)).status).toBe(204);
    expect((await remove("no-such-card")).status).toBe(204);
  });

  it("saves the Arabic hint of the card", async () => {
    const { store } = context();
    const id = openCard(store, { rank: 10 })!;
    const res = await edit(id, { hintAr: "ليس كَتَب" });
    expect(res.status).toBe(200);
    expect(store.getNote(id)!.hintAr).toBe("ليس كَتَب");
  });
});
