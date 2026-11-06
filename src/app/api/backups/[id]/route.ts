import { context, fail, handle } from "@/lib/server";
import { deleteSnapshot, listSnapshots } from "@/lib/snapshots";

export const dynamic = "force-dynamic";
type Ctx = { params: Promise<{ id: string }> };

export const DELETE = (_req: Request, { params }: Ctx) =>
  handle(async () => {
    const { id } = await params;
    const { store } = context();
    if (!listSnapshots(store.root).some((s) => s.id === id)) return fail("There is no such backup.", 404);
    deleteSnapshot(store.root, id);
    return new Response(null, { status: 204 });
  });
