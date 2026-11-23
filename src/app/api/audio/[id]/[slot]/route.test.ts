import { describe, expect, it } from "vitest";
import { cardForRank } from "@/lib/lexicon";
import { context } from "@/lib/server";
import { mp3, tempWorkspace, withParams } from "@/lib/testing/workspace";
import { DELETE } from "./route";

const remove = (id: string, slot: string) =>
  DELETE(new Request(`http://127.0.0.1/api/audio/${id}/${slot}`, { method: "DELETE" }), withParams({ id, slot }));

describe("removing a clip", () => {
  tempWorkspace();

  it("answers with the card as it is without the clip", async () => {
    const { store } = context();
    const note = store.createNote(cardForRank(2)!);
    store.setAudio(note.id, "word", mp3, "user");
    const response = await remove(note.id, "word");
    expect(response.status).toBe(200);
    const body = await response.json();
    expect(body.note.id).toBe(note.id);
    expect(body.note.audio.word).toBeUndefined();
    expect(store.getNote(note.id)!.audio.word).toBeUndefined();
  });

  it("says there is no such card for an unknown id", async () => {
    const response = await remove("no-such-card", "word");
    expect(response.status).toBe(404);
    expect(await response.json()).toEqual({ error: "There is no such card." });
  });

  it("refuses a slot that is not the card's, with the store's message", async () => {
    const note = context().store.createNote(cardForRank(2)!);
    const response = await remove(note.id, "not-a-slot");
    expect(response.status).toBe(400);
    expect(await response.json()).toEqual({ error: "That audio slot does not belong to this card." });
  });
});
