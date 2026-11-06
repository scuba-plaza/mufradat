import fs from "node:fs";
import { NextResponse } from "next/server";
import { NOT_A_BACKUP } from "@/lib/backup-zip";
import { refuseCrossSite } from "@/lib/origin";
import { context, fail, handle } from "@/lib/server";
import { importBackup, incomingFile } from "@/lib/snapshots";
import { MAX_IMPORT_BYTES, saveBody, TooLargeError } from "@/lib/upload";

export const dynamic = "force-dynamic";

export const POST = (req: Request) =>
  handle(async () => {
    const refused = refuseCrossSite(req);
    if (refused) return refused;
    if (Number(req.headers.get("content-length")) > MAX_IMPORT_BYTES) return fail(new TooLargeError().message, 413);
    if (!req.body) return fail(NOT_A_BACKUP);
    const { store } = context();
    const incoming = incomingFile(store.root);
    try {
      await saveBody(req.body, incoming, MAX_IMPORT_BYTES);
      return NextResponse.json({ snapshot: importBackup(store.root, incoming) });
    } catch (e) {
      return fail((e as Error).message, e instanceof TooLargeError ? 413 : 400);
    } finally {
      fs.rmSync(incoming, { force: true });
    }
  });
