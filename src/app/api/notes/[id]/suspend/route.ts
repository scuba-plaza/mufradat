import { NextResponse } from "next/server";
import { context, handle } from "@/lib/server";
import type { IdContext } from "../../../route-helpers";

export const dynamic = "force-dynamic";

export const POST = (_req: Request, { params }: IdContext) =>
  handle(async () => NextResponse.json({ note: context().store.suspend((await params).id) }));

export const DELETE = (_req: Request, { params }: IdContext) =>
  handle(async () => NextResponse.json({ note: context().store.unsuspend((await params).id) }));
