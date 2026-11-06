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
      return fail((e as Error).message);
    }
    const delivered = () => {
      try {
        recordDownload(store.root);
      } catch (error) {
        console.error(error);
      }
    };
    return new Response(backupZipStream(entries, delivered), {
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
