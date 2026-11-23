import { NextResponse } from "next/server";
import { ApiError } from "@/lib/api-error";
import { generateExample } from "@/lib/mistral";
import { context, handle, readJson } from "@/lib/server";
import { noSuchCard, noteOf } from "../route-helpers";

/** Asks Mistral once for an example. A failure is kept on the card and nothing else changes. */
export const POST = (req: Request) =>
  handle(async () => {
    const { id } = await readJson(req);
    const { cfg, store } = context();
    const note = noteOf(store, id);
    if (!note) return noSuchCard();
    try {
      const { example, exampleEn } = await generateExample(note, cfg);
      return NextResponse.json({ note: store.applyGeneratedExample(note.id, example, exampleEn) });
    } catch (e) {
      if (!(e instanceof ApiError)) throw e;
      return NextResponse.json({ note: store.setExampleError(note.id, e.message) });
    }
  });
