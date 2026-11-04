import { NextResponse } from "next/server";
import { ApiError } from "@/lib/api-error";
import { generateExample } from "@/lib/mistral";
import { context, fail, handle, readJson } from "@/lib/server";

/** Asks Mistral once for an example. A failure is kept on the card and nothing else changes. */
export const POST = (req: Request) =>
  handle(async () => {
    const { id } = await readJson(req);
    const { cfg, store } = context();
    const note = typeof id === "string" ? store.getNote(id) : null;
    if (!note) return fail("There is no such card.", 404);
    try {
      const { example, exampleEn } = await generateExample(note, cfg);
      return NextResponse.json({ note: store.applyGeneratedExample(note.id, example, exampleEn) });
    } catch (e) {
      if (!(e instanceof ApiError)) throw e;
      return NextResponse.json({ note: store.setExampleError(note.id, e.message) });
    }
  });
