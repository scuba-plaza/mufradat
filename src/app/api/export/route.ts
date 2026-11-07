import fs from "node:fs";
import { buildApkg } from "@/lib/anki";
import { audioPath, MissingClipError, type ClipPlace } from "@/lib/audio";
import { context, fail, handle } from "@/lib/server";
import { approvedIds, view } from "@/lib/words";

export const dynamic = "force-dynamic";

export const GET = (req: Request) =>
  handle(() => {
    const { cfg, store } = context();
    const q = new URL(req.url).searchParams.get("q") ?? "";
    const deck = store.allNotes().filter((n) => n.status === "approved");
    let notes = deck;
    if (q.trim() !== "") {
      const found = view(store, q);
      if (found.error) return fail(found.error);
      const wanted = new Set(approvedIds(found.rows));
      notes = notes.filter((n) => wanted.has(n.id));
    }
    if (notes.length === 0) return fail(q.trim() === "" ? "There are no approved cards to export." : "No approved card matches this search.");
    const read = (key: string, place: ClipPlace) => {
      const file = audioPath(store.root, key);
      if (!fs.existsSync(file)) throw new MissingClipError(place);
      return fs.readFileSync(file);
    };
    let apkg: Buffer<ArrayBuffer>;
    try {
      apkg = buildApkg(notes, read, cfg, deck);
    } catch (e) {
      if (e instanceof MissingClipError) return fail(e.message);
      throw e;
    }
    const slug = q.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "").slice(0, 30);
    const name = `arabic-deck-${new Date().toISOString().slice(0, 10)}${slug ? `-${slug}` : ""}.apkg`;
    return new Response(apkg, {
      headers: {
        "Content-Type": "application/octet-stream",
        "Content-Disposition": `attachment; filename="${name}"`,
      },
    });
  });
