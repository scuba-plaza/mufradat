import { NextResponse } from "next/server";
import { context, fail, handle } from "@/lib/server";
import { neighbour, rowHref } from "@/lib/words";

export const dynamic = "force-dynamic";

export const GET = (req: Request) =>
  handle(() => {
    const params = new URL(req.url).searchParams;
    const direction = params.get("dir");
    if (direction !== "previous" && direction !== "next") return fail("dir must be previous or next.");
    const q = (params.get("q") ?? "").trim();
    const row = neighbour(context().store, params.get("id") ?? "", q, direction);
    return NextResponse.json({ href: row ? rowHref(row, q) : null });
  });
