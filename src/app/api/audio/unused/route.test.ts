import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { audioKey, audioPath } from "@/lib/audio";
import { context } from "@/lib/server";
import { openCard } from "@/lib/words";
import { GET as playClip } from "../[id]/[slot]/route";
import { DELETE, GET } from "./route";

const bytes = (n: number) => Buffer.from([0xff, 0xfb, 0x90, 0x00, n, n, n, n]);

describe("the audio files that no clip uses, through the API", () => {
  let home: string;

  beforeEach(() => {
    home = fs.mkdtempSync(path.join(os.tmpdir(), "mufradat-unused-route-"));
    vi.stubEnv("XDG_CONFIG_HOME", path.join(home, "config"));
    vi.stubEnv("MUFRADAT_WORKSPACE", path.join(home, "workspace"));
  });
  afterEach(() => {
    vi.unstubAllEnvs();
    fs.rmSync(home, { recursive: true, force: true });
  });

  function oneUsedAndOneUnused() {
    const { store } = context();
    const kept = openCard(store, { rank: 10 })!;
    const gone = openCard(store, { rank: 11 })!;
    store.setAudio(kept, "word", bytes(1), "tts");
    store.setAudio(gone, "word", bytes(2), "tts");
    store.removeAudio(gone, "word");
    return { store, kept, orphan: audioPath(store.root, audioKey(bytes(2))) };
  }

  it("reports the unused files without deleting them", async () => {
    const { orphan } = oneUsedAndOneUnused();
    const res = await GET();
    expect(res.status).toBe(200);
    expect(await res.json()).toEqual({ files: 1, bytes: 8 });
    expect(fs.existsSync(orphan)).toBe(true);
  });

  it("deletes the unused files, says what it deleted, and leaves the clips in use playing", async () => {
    const { kept, orphan } = oneUsedAndOneUnused();
    const res = await DELETE();
    expect(res.status).toBe(200);
    expect(await res.json()).toEqual({ files: 1, bytes: 8 });
    expect(fs.existsSync(orphan)).toBe(false);
    expect(await (await GET()).json()).toEqual({ files: 0, bytes: 0 });
    const played = await playClip(new Request(`http://127.0.0.1/api/audio/${kept}/word`), { params: Promise.resolve({ id: kept, slot: "word" }) });
    expect(played.status).toBe(200);
    expect(Buffer.from(await played.arrayBuffer())).toEqual(bytes(1));
  });

  it("answers with nothing to delete in a workspace that has no audio", async () => {
    expect(await (await DELETE()).json()).toEqual({ files: 0, bytes: 0 });
  });
});
