import { NextResponse } from "next/server";
import { context, handle } from "@/lib/server";
import { restoreSnapshot } from "@/lib/snapshots";
import { hasBackup, noSuchBackup, refusal, type IdContext } from "../../../route-helpers";

export const dynamic = "force-dynamic";

export const POST = (_req: Request, { params }: IdContext) =>
  handle(async () => {
    const { id } = await params;
    const { store } = context();
    if (!hasBackup(store.root, id)) return noSuchBackup();
    try {
      restoreSnapshot(store, id);
    } catch (e) {
      return refusal(e, "The backup was not restored.");
    }
    return NextResponse.json({ ok: true });
  });
