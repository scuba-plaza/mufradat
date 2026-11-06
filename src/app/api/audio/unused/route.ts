import { NextResponse } from "next/server";
import { context, handle } from "@/lib/server";

export const dynamic = "force-dynamic";

export const GET = () => handle(() => NextResponse.json(context().store.unusedAudio()));

export const DELETE = () => handle(() => NextResponse.json(context().store.deleteUnusedAudio()));
