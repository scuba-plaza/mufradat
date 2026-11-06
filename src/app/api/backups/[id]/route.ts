import { backupZipStream, type BackupEntry } from "@/lib/backup-zip";
import { context, fail, handle } from "@/lib/server";
import { backupEntries, deleteSnapshot, listSnapshots, recordDownload } from "@/lib/snapshots";

export const dynamic = "force-dynamic";
type Ctx = { params: Promise<{ id: string }> };

export const GET = (_req: Request, { params }: Ctx) =>
  handle(async () => {
    const { id } = await params;
    const { store } = context();
    if (!listSnapshots(store.root).some((s) => s.id === id)) return fail("There is no such backup.", 404);
    let entries: BackupEntry[];
    try {
      entries = backupEntries(store.root, id);
    } catch (e) {
      return fail(e instanceof Error ? e.message : "The backup could not be read.");
    }
    recordDownload(store.root);
    return new Response(backupZipStream(entries), {
      headers: { "Content-Type": "application/zip", "Content-Disposition": `attachment; filename="mufradat-backup-${id}.zip"` },
    });
  });

export const DELETE = (_req: Request, { params }: Ctx) =>
  handle(async () => {
    const { id } = await params;
    const { store } = context();
    if (!listSnapshots(store.root).some((s) => s.id === id)) return fail("There is no such backup.", 404);
    deleteSnapshot(store.root, id);
    return new Response(null, { status: 204 });
  });
