import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cardForRank } from "@/lib/lexicon";
import { context } from "@/lib/server";
import { DELETE } from "./route";

const mp3 = Buffer.from([0xff, 0xfb, 0x90, 0x00, 1, 2, 3, 4]);

function remove(id: string, slot: string) {
  return DELETE(new Request(`http://127.0.0.1/api/audio/${id}/${slot}`, { method: "DELETE" }), {
    params: Promise.resolve({ id, slot }),
  });
}

describe("removing a clip", () => {
  beforeEach(() => {
    const home = fs.mkdtempSync(path.join(os.tmpdir(), "mufradat-route-"));
    vi.stubEnv("XDG_CONFIG_HOME", path.join(home, "config"));
    vi.stubEnv("MUFRADAT_WORKSPACE", path.join(home, "workspace"));
  });
  afterEach(() => {
    vi.unstubAllEnvs();
  });

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
