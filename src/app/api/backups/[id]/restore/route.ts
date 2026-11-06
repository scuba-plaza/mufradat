import { NextResponse } from "next/server";
import { context, fail, handle } from "@/lib/server";
import { listSnapshots, restoreSnapshot } from "@/lib/snapshots";

export const dynamic = "force-dynamic";
type Ctx = { params: Promise<{ id: string }> };

export const POST = (_req: Request, { params }: Ctx) =>
  handle(async () => {
    const { id } = await params;
    const { store } = context();
    if (!listSnapshots(store.root).some((s) => s.id === id)) return fail("There is no such backup.", 404);
    try {
      restoreSnapshot(store, id);
    } catch (e) {
      return fail(e instanceof Error ? e.message : "The backup was not restored.");
    }
    return NextResponse.json({ ok: true });
  });
