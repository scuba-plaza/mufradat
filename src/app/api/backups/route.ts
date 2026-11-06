import { NextResponse } from "next/server";
import { context, handle } from "@/lib/server";
import { createSnapshot, listSnapshots, snapshotUsage } from "@/lib/snapshots";

export const dynamic = "force-dynamic";

export const GET = () =>
  handle(() => {
    const { store } = context();
    return NextResponse.json({
      snapshots: listSnapshots(store.root),
      usage: snapshotUsage(store.root),
      check: store.check(),
    });
  });

export const POST = () => handle(() => NextResponse.json({ snapshot: createSnapshot(context().store, "manual") }));
