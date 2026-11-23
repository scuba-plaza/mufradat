import { backupZipStream, type BackupEntry } from "@/lib/backup-zip";
import { context, fail, handle } from "@/lib/server";
import { backupEntries, deleteSnapshot, recordDownload } from "@/lib/snapshots";
import { hasBackup, noSuchBackup, type IdContext } from "../../route-helpers";

export const dynamic = "force-dynamic";

export const GET = (_req: Request, { params }: IdContext) =>
  handle(async () => {
    const { id } = await params;
    const { store } = context();
    if (!hasBackup(store.root, id)) return noSuchBackup();
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

export const DELETE = (_req: Request, { params }: IdContext) =>
  handle(async () => {
    const { id } = await params;
    const { store } = context();
    if (!hasBackup(store.root, id)) return noSuchBackup();
    deleteSnapshot(store.root, id);
    return new Response(null, { status: 204 });
  });
