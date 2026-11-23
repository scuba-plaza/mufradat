import { NextResponse } from "next/server";
import { context, handle } from "@/lib/server";
import type { IdContext } from "../../../route-helpers";

export const POST = (_req: Request, { params }: IdContext) =>
  handle(async () => {
    const result = context().store.approve((await params).id);
    return result.ok
      ? NextResponse.json({ note: result.note })
      : NextResponse.json({ errors: result.errors }, { status: 422 });
  });
