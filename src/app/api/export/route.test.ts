import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { audioKey, audioPath } from "@/lib/audio";
import { context } from "@/lib/server";
import { openCard } from "@/lib/words";
import { POST } from "./route";

const mp3 = Buffer.from([0xff, 0xfb, 0x90, 0x00, 1, 2, 3, 4]);

describe("exporting through the API", () => {
  let home: string;

  beforeEach(() => {
    home = fs.mkdtempSync(path.join(os.tmpdir(), "mufradat-export-route-"));
    vi.stubEnv("XDG_CONFIG_HOME", path.join(home, "config"));
    vi.stubEnv("MUFRADAT_WORKSPACE", path.join(home, "workspace"));
  });
  afterEach(() => {
    vi.unstubAllEnvs();
    fs.rmSync(home, { recursive: true, force: true });
  });

  it("names the card and the slot when a clip's file is missing", async () => {
    const { store } = context();
    const id = openCard(store, { rank: 10 })!;
    store.setAudio(id, "word", mp3, "tts");
    expect(store.approve(id).ok).toBe(true);
    fs.rmSync(audioPath(store.root, audioKey(mp3)));
    const res = await POST();
    expect(res.status).toBe(400);
    expect(await res.json()).toEqual({
      error: `The audio for ${store.getNote(id)!.lemma} (word) is missing from the workspace. Generate or upload it again on the card.`,
    });
  });

  it("still exports when every clip is there", async () => {
    const { store } = context();
    const id = openCard(store, { rank: 10 })!;
    store.setAudio(id, "word", mp3, "tts");
    expect(store.approve(id).ok).toBe(true);
    const res = await POST();
    expect(res.status).toBe(200);
    expect(await res.json()).toMatchObject({ cards: 1 });
  });
});
