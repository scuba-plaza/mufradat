import { NextResponse } from "next/server";
import { context, fail, handle, readJson } from "@/lib/server";
import { parseTags } from "@/lib/tags";
import type { IdContext } from "../../../route-helpers";

export const dynamic = "force-dynamic";

export const PUT = (req: Request, { params }: IdContext) =>
  handle(async () => {
    const parsed = parseTags((await readJson(req)).tags);
    if ("error" in parsed) return fail(parsed.error);
    return NextResponse.json({ note: context().store.setTags((await params).id, parsed.tags) });
  });
