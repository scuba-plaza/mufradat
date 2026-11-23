import { describe, expect, it } from "vitest";
import { context } from "@/lib/server";
import { openCard } from "@/lib/words";
import { tempWorkspace, withParams } from "@/lib/testing/workspace";
import { DELETE, POST } from "./route";

const suspend = (id: string) => POST(new Request(`http://127.0.0.1/api/notes/${id}/suspend`, { method: "POST" }), withParams({ id }));
const unsuspend = (id: string) => DELETE(new Request(`http://127.0.0.1/api/notes/${id}/suspend`, { method: "DELETE" }), withParams({ id }));

describe("suspending a card through the API", () => {
  tempWorkspace();

  it("suspends the card and takes its approval back", async () => {
    const { store } = context();
    const id = openCard(store, { rank: 10 })!;
    store.approve(id);
    const res = await suspend(id);
    expect(res.status).toBe(200);
    const { note } = (await res.json()) as { note: { suspended: boolean; status: string } };
    expect(note).toMatchObject({ suspended: true, status: "draft" });
    expect(store.getNote(id)!.suspended).toBe(true);
  });

  it("unsuspends the card and leaves it a draft", async () => {
    const { store } = context();
    const id = openCard(store, { rank: 10 })!;
    await suspend(id);
    const res = await unsuspend(id);
    expect(res.status).toBe(200);
    const { note } = (await res.json()) as { note: { suspended: boolean; status: string } };
    expect(note).toMatchObject({ suspended: false, status: "draft" });
    expect(store.getNote(id)!.suspended).toBe(false);
  });

  it("says there is no such card", async () => {
    context();
    expect(await suspend("no-such-card")).toMatchObject({ status: 500 });
    expect(await unsuspend("no-such-card")).toMatchObject({ status: 500 });
  });
});
