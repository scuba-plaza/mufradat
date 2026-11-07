import { NextResponse } from "next/server";
import { context, fail, handle, readJson } from "@/lib/server";
import { parseTags } from "@/lib/tags";

export const dynamic = "force-dynamic";
type Ctx = { params: Promise<{ id: string }> };

export const PUT = (req: Request, { params }: Ctx) =>
  handle(async () => {
    const parsed = parseTags((await readJson(req)).tags);
    if ("error" in parsed) return fail(parsed.error);
    return NextResponse.json({ note: context().store.setTags((await params).id, parsed.tags) });
  });
