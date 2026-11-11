import { NextResponse } from "next/server";
import { context, handle } from "@/lib/server";

export const dynamic = "force-dynamic";
type Ctx = { params: Promise<{ id: string }> };

export const POST = (_req: Request, { params }: Ctx) =>
  handle(async () => NextResponse.json({ note: context().store.suspend((await params).id) }));

export const DELETE = (_req: Request, { params }: Ctx) =>
  handle(async () => NextResponse.json({ note: context().store.unsuspend((await params).id) }));
