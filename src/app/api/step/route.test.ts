import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { context } from "@/lib/server";
import { openCard } from "@/lib/words";
import { GET } from "./route";

async function step(id: string, dir: string, q?: string) {
  const params = new URLSearchParams({ id, dir, ...(q === undefined ? {} : { q }) });
  const res = await GET(new Request(`http://127.0.0.1/api/step?${params}`));
  return { status: res.status, body: await res.json() };
}

describe("stepping from a card to the next word of its list", () => {
  let home: string;

  beforeEach(() => {
    home = fs.mkdtempSync(path.join(os.tmpdir(), "mufradat-step-"));
    vi.stubEnv("XDG_CONFIG_HOME", path.join(home, "config"));
    vi.stubEnv("MUFRADAT_WORKSPACE", path.join(home, "workspace"));
  });
  afterEach(() => {
    vi.unstubAllEnvs();
    fs.rmSync(home, { recursive: true, force: true });
  });

  it("answers with the address of the next word of the frequency list when there is no query", async () => {
    const id = openCard(context().store, { rank: 10 })!;
    expect(await step(id, "next")).toEqual({ status: 200, body: { href: "/open?rank=11" } });
    expect(await step(id, "previous")).toEqual({ status: 200, body: { href: "/open?rank=9" } });
  });

  it("answers with the card of a neighbour that has one, and carries the query on", async () => {
    const { store } = context();
    const [, second] = [1, 2, 3].map((rank) => openCard(store, { rank })!);
    const third = store.findNote("عَلَى", "prep")!;
    const { body } = await step(second, "next", "pos:prep rank:<12");
    expect(body).toEqual({ href: `/cards/${third}?q=pos%3Aprep+rank%3A%3C12` });
  });

  it("answers with no address at the end of the list", async () => {
    const id = openCard(context().store, { rank: 5000 })!;
    expect(await step(id, "next")).toEqual({ status: 200, body: { href: null } });
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
    expect((await step(tenth, "next", "rank:<14")).body).toEqual({ href: "/open?rank=12&q=rank%3A%3C14" });
  });

  it("refuses a direction that is neither previous nor next", async () => {
    const id = openCard(context().store, { rank: 10 })!;
    expect((await step(id, "sideways")).status).toBe(400);
  });

  it("refuses a request that names no direction", async () => {
    const id = openCard(context().store, { rank: 10 })!;
    const res = await GET(new Request(`http://127.0.0.1/api/step?id=${id}`));
    expect(res.status).toBe(400);
  });

  it("answers with no address for a query it cannot read and for a card that does not exist", async () => {
    const id = openCard(context().store, { rank: 10 })!;
    expect(await step(id, "next", "pos:animal")).toEqual({ status: 200, body: { href: null } });
    expect(await step("no-such-card", "next")).toEqual({ status: 200, body: { href: null } });
    expect(await step("no-such-card", "next", "pos:noun")).toEqual({ status: 200, body: { href: null } });
  });
});
