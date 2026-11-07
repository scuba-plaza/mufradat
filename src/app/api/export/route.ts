import fs from "node:fs";
import { buildApkg } from "@/lib/anki";
import { audioPath, MissingClipError, type ClipPlace } from "@/lib/audio";
import { context, fail, handle } from "@/lib/server";

export const dynamic = "force-dynamic";

export const GET = () =>
  handle(() => {
    const { cfg, store } = context();
    const notes = store.approvedNotes();
    if (notes.length === 0) return fail("There are no approved cards to export.");
    const read = (key: string, place: ClipPlace) => {
      const file = audioPath(store.root, key);
      if (!fs.existsSync(file)) throw new MissingClipError(place);
      return fs.readFileSync(file);
    };
    let apkg: Buffer<ArrayBuffer>;
    try {
      apkg = buildApkg(notes, read, cfg);
    } catch (e) {
      if (e instanceof MissingClipError) return fail(e.message);
      throw e;
    }
    const name = `arabic-deck-${new Date().toISOString().slice(0, 10)}.apkg`;
    return new Response(apkg, {
      headers: {
        "Content-Type": "application/octet-stream",
        "Content-Disposition": `attachment; filename="${name}"`,
      },
    });
  });
