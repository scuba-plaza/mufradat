import { NextResponse } from "next/server";
import { context, handle } from "@/lib/server";
import { createSnapshot, lastDownload, listSnapshots, snapshotUsage } from "@/lib/snapshots";

export const dynamic = "force-dynamic";

export const GET = () =>
  handle(() => {
    const { store } = context();
    return NextResponse.json({
      snapshots: listSnapshots(store.root),
      usage: snapshotUsage(store.root),
      check: store.check(),
      lastDownloadedAt: lastDownload(store.root),
    });
  });

export const POST = () => handle(() => NextResponse.json({ snapshot: createSnapshot(context().store, "manual") }));
