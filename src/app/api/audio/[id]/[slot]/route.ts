import fs from "node:fs";
import { NextResponse } from "next/server";
import { audioPath } from "@/lib/audio";
import { context, fail, handle } from "@/lib/server";

export const dynamic = "force-dynamic";
type Ctx = { params: Promise<{ id: string; slot: string }> };

export const GET = (_req: Request, { params }: Ctx) =>
  handle(async () => {
    const { id, slot } = await params;
    const { store } = context();
    const key = store.getNote(id)?.audio[slot]?.key;
    if (!key) return fail("There is no audio for that field.", 404);
    return new Response(fs.readFileSync(audioPath(store.root, key)), { headers: { "Content-Type": "audio/mpeg", "Cache-Control": "no-store" } });
  });

/** The request body is the MP3 file itself. */
export const PUT = (req: Request, { params }: Ctx) =>
  handle(async () => {
    const { id, slot } = await params;
    const bytes = Buffer.from(await req.arrayBuffer());
    try {
      return NextResponse.json({ note: context().store.setAudio(id, slot, bytes, "user") });
    } catch (e) {
      return fail(e instanceof Error ? e.message : "The audio was not saved.");
    }
  });
