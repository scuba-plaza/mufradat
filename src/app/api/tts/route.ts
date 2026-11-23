import { NextResponse } from "next/server";
import { ApiError } from "@/lib/api-error";
import { context, fail, handle, readJson } from "@/lib/server";
import { synthesize } from "@/lib/tts";
import type { Note } from "@/lib/types";
import { plainExample } from "@/lib/validate";
import { noSuchCard, noteOf } from "../route-helpers";

function clipText(note: Note, slot: string): string | undefined {
  if (slot === "word") return note.lemma;
  if (slot === "example") return plainExample(note.example);
  return note.forms.find((f) => f.id === slot)?.arabic;
}

/** Makes one clip with Google. A failure is kept on the card and the old clip stays. */
export const POST = (req: Request) =>
  handle(async () => {
    const { id, slot } = await readJson(req);
    const { cfg, store } = context();
    const note = noteOf(store, id);
    if (!note || typeof slot !== "string") return noSuchCard();
    const text = clipText(note, slot);
    if (!text?.trim()) return fail("There is no Arabic text for that clip.");
    try {
      return NextResponse.json({ note: store.setAudio(note.id, slot, await synthesize(text, cfg), "tts") });
    } catch (e) {
      if (!(e instanceof ApiError)) throw e;
      return NextResponse.json({ note: store.setAudioError(note.id, slot, e.message) });
    }
  });
