import { describe, expect, it, vi } from "vitest";
import { createPlayer } from "./player";

class FakeAudio {
  static all: FakeAudio[] = [];
  onended: (() => void) | null = null;
  onerror: (() => void) | null = null;
  onpause: (() => void) | null = null;
  plays = 0;
  constructor(readonly url: string) {
    FakeAudio.all.push(this);
  }
  play() {
    this.plays++;
    return Promise.resolve();
  }
}

const setup = () => {
  FakeAudio.all = [];
  const player = createPlayer((url) => new FakeAudio(url));
  return { player, audio: FakeAudio.all };
};

describe("playing clips one after another", () => {
  it("plays a clip at once when nothing is playing, and says which one", () => {
    const { player, audio } = setup();
    player.enqueue({ id: "a/word", url: "/a", onError: vi.fn() });
    expect(audio.map((x) => [x.url, x.plays])).toEqual([["/a", 1]]);
    expect(player.playing()).toBe("a/word");
  });

  it("keeps a clip waiting while another plays, and plays it when that one ends", () => {
    const { player, audio } = setup();
    player.enqueue({ id: "a/word", url: "/a", onError: vi.fn() });
    player.enqueue({ id: "a/example", url: "/b", onError: vi.fn() });
    expect(audio.map((x) => x.url)).toEqual(["/a"]);
    audio[0].onended!();
    expect(audio.map((x) => x.url)).toEqual(["/a", "/b"]);
    expect(player.playing()).toBe("a/example");
    audio[1].onended!();
    expect(player.playing()).toBeNull();
  });

  it("tells its listeners when the playing clip changes", () => {
    const { player, audio } = setup();
    const heard = vi.fn();
    player.subscribe(heard);
    player.enqueue({ id: "a/word", url: "/a", onError: vi.fn() });
    audio[0].onended!();
    expect(heard).toHaveBeenCalledTimes(2);
  });

  it("reports a clip that cannot be played once, and goes on with the next", () => {
    const { player, audio } = setup();
    const failed = vi.fn();
    player.enqueue({ id: "a/word", url: "/a", onError: failed });
    player.enqueue({ id: "a/example", url: "/b", onError: vi.fn() });
    audio[0].onerror!();
    audio[0].onerror!();
    expect(failed).toHaveBeenCalledTimes(1);
    expect(player.playing()).toBe("a/example");
  });

  it("treats a refused play like a clip that cannot be played", async () => {
    FakeAudio.all = [];
    const refuse = class extends FakeAudio {
      play() {
        return Promise.reject(new Error("blocked"));
      }
    };
    const player = createPlayer((url) => new refuse(url));
    const failed = vi.fn();
    player.enqueue({ id: "a/word", url: "/a", onError: failed });
    await Promise.resolve();
    await Promise.resolve();
    expect(failed).toHaveBeenCalledTimes(1);
    expect(player.playing()).toBeNull();
  });

  it("goes on with the next clip when the playing one is paused from outside, as by a phone call", () => {
    const { player, audio } = setup();
    player.enqueue({ id: "a/word", url: "/a", onError: vi.fn() });
    player.enqueue({ id: "a/example", url: "/b", onError: vi.fn() });
    audio[0].onpause!();
    expect(player.playing()).toBe("a/example");
  });

  it("does not stay stuck when the sound cannot even be made", () => {
    const player = createPlayer(() => {
      throw new Error("no audio here");
    });
    const failed = vi.fn();
    player.enqueue({ id: "a/word", url: "/a", onError: failed });
    expect(failed).toHaveBeenCalledTimes(1);
    expect(player.playing()).toBeNull();
  });

  it("keeps a clip waiting only once, however often it is asked for", () => {
    const { player, audio } = setup();
    player.enqueue({ id: "a/word", url: "/a", onError: vi.fn() });
    player.enqueue({ id: "a/example", url: "/b", onError: vi.fn() });
    player.enqueue({ id: "a/example", url: "/c", onError: vi.fn() });
    audio[0].onended!();
    audio[1].onended!();
    expect(audio.map((x) => x.url)).toEqual(["/a", "/c"]);
    expect(player.playing()).toBeNull();
  });

  it("drops a waiting clip that is forgotten", () => {
    const { player, audio } = setup();
    player.enqueue({ id: "a/word", url: "/a", onError: vi.fn() });
    player.enqueue({ id: "a/example", url: "/b", onError: vi.fn() });
    player.forget("a/example");
    audio[0].onended!();
    expect(audio).toHaveLength(1);
    expect(player.playing()).toBeNull();
  });
});
