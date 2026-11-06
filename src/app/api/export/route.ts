import fs from "node:fs";
import path from "node:path";
import { NextResponse } from "next/server";
import { buildApkg } from "@/lib/anki";
import { audioPath, MissingClipError, type ClipPlace } from "@/lib/audio";
import { context, fail, handle } from "@/lib/server";

export const dynamic = "force-dynamic";

export const POST = () =>
  handle(() => {
    const { cfg, store } = context();
    const notes = store.approvedNotes();
    if (notes.length === 0) return fail("There are no approved cards to export.");
    const read = (key: string, place: ClipPlace) => {
      const file = audioPath(store.root, key);
      if (!fs.existsSync(file)) throw new MissingClipError(place);
      return fs.readFileSync(file);
    };
    let apkg: Buffer;
    try {
      apkg = buildApkg(notes, read, cfg);
    } catch (e) {
      if (e instanceof MissingClipError) return fail(e.message);
      throw e;
    }
    const dir = path.join(store.root, "exports");
    fs.mkdirSync(dir, { recursive: true });
    const name = `arabic-deck-${new Date().toISOString().slice(0, 10)}.apkg`;
    fs.writeFileSync(path.join(dir, name), apkg);
    return NextResponse.json({ file: name, path: path.join(dir, name), cards: notes.length });
  });

export const GET = (req: Request) =>
  handle(() => {
    const name = new URL(req.url).searchParams.get("file") ?? "";
    if (!/^arabic-deck-\d{4}-\d{2}-\d{2}\.apkg$/.test(name)) return fail("Unknown export.", 404);
    const file = path.join(context().store.root, "exports", name);
    if (!fs.existsSync(file)) return fail("Unknown export.", 404);
    return new Response(fs.readFileSync(file), {
      headers: { "Content-Type": "application/octet-stream", "Content-Disposition": `attachment; filename="${name}"` },
    });
  });
