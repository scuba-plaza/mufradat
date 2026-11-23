import { describe, expect, it } from "vitest";
import { context } from "@/lib/server";
import { openCard } from "@/lib/words";
import { tempWorkspace } from "@/lib/testing/workspace";
import { GET } from "./route";

async function step(id: string, dir: string, q?: string) {
  const params = new URLSearchParams({ id, dir, ...(q === undefined ? {} : { q }) });
  const res = await GET(new Request(`http://127.0.0.1/api/step?${params}`));
  return { status: res.status, body: await res.json() };
}

describe("stepping from a card to the next word of its list", () => {
  tempWorkspace();

  it("answers with the address of the next word of the frequency list when there is no query", async () => {
    const id = openCard(context().store, { rank: 10 })!;
    expect(await step(id, "next")).toEqual({ status: 200, body: { href: "/open?rank=11" } });
    expect(await step(id, "previous")).toEqual({ status: 200, body: { href: "/open?rank=9" } });
  });

  it("answers with the card of a neighbour that has one, and carries the query on", async () => {
    const { store } = context();
    const [, second] = [1, 2, 3].map((rank) => openCard(store, { rank })!);
    const third = store.findNote("عَلَى", "prep")!;
    const { body } = await step(second, "next", "pos:prep rank:1-11");
    expect(body).toEqual({ href: `/cards/${third}?q=pos%3Aprep+rank%3A1-11` });
  });

  it("steps over a suspended card, and walks the cards of a status:suspended queue", async () => {
    const { store } = context();
    const tenth = openCard(store, { rank: 10 })!;
    const eleventh = openCard(store, { rank: 11 })!;
    const thirteenth = openCard(store, { rank: 13 })!;
    store.suspend(eleventh);
    store.suspend(thirteenth);
    expect((await step(tenth, "next")).body).toEqual({ href: "/open?rank=12" });
    expect((await step(tenth, "previous")).body).toEqual({ href: "/open?rank=9" });
    expect((await step(eleventh, "next", "status:suspended")).body).toEqual({ href: `/cards/${thirteenth}?q=status%3Asuspended` });
    expect((await step(thirteenth, "previous", "status:suspended")).body).toEqual({ href: `/cards/${eleventh}?q=status%3Asuspended` });
    expect((await step(tenth, "next", "rank:1-13")).body).toEqual({ href: "/open?rank=12&q=rank%3A1-13" });
  });

  it("refuses a direction that is neither previous nor next, or none at all", async () => {
    const id = openCard(context().store, { rank: 10 })!;
    expect((await step(id, "sideways")).status).toBe(400);
    expect((await GET(new Request(`http://127.0.0.1/api/step?id=${id}`))).status).toBe(400);
  });

  it("answers with no address at the end of the list, for a query it cannot read and for a card that does not exist", async () => {
    const last = openCard(context().store, { rank: 5000 })!;
    expect(await step(last, "next")).toEqual({ status: 200, body: { href: null } });
    const id = openCard(context().store, { rank: 10 })!;
    expect(await step(id, "next", "pos:animal")).toEqual({ status: 200, body: { href: null } });
    expect(await step("no-such-card", "next")).toEqual({ status: 200, body: { href: null } });
    expect(await step("no-such-card", "next", "pos:noun")).toEqual({ status: 200, body: { href: null } });
  });
});
