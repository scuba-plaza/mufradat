import { NextResponse } from "next/server";
import { ApiError } from "@/lib/api-error";
import { plainExample } from "@/lib/validate";
import { context, fail, handle, readJson } from "@/lib/server";
import { synthesize } from "@/lib/tts";

/** Makes one clip with Google. A failure is kept on the card and the old clip stays. */
export const POST = (req: Request) =>
  handle(async () => {
    const { id, slot } = await readJson(req);
    const { cfg, store } = context();
    const note = typeof id === "string" ? store.getNote(id) : null;
    if (!note || typeof slot !== "string") return fail("There is no such card.", 404);
    const text =
      slot === "word" ? note.lemma : slot === "example" ? plainExample(note.example) : note.forms.find((f) => f.id === slot)?.arabic;
    if (!text?.trim()) return fail("There is no Arabic text for that clip.");
    try {
      return NextResponse.json({ note: store.setAudio(note.id, slot, await synthesize(text, cfg), "tts") });
    } catch (e) {
      if (!(e instanceof ApiError)) throw e;
      return NextResponse.json({ note: store.setAudioError(note.id, slot, e.message) });
    }
  });
