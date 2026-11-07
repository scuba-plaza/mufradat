export interface Clip {
  id: string;
  url: string;
  onError: () => void;
}

interface Sound {
  onended: ((event: Event) => unknown) | null;
  onerror: ((event: Event | string) => unknown) | null;
  play(): Promise<void>;
}

export function createPlayer(make: (url: string) => Sound) {
  let current: Clip | null = null;
  const waiting: Clip[] = [];
  const listeners = new Set<() => void>();
  const tell = () => listeners.forEach((listen) => listen());

  function start(clip: Clip) {
    current = clip;
    tell();
    let over = false;
    const finish = (failed: boolean) => {
      if (over) return;
      over = true;
      if (failed) clip.onError();
      current = null;
      const next = waiting.shift();
      if (next) start(next);
      else tell();
    };
    const sound = make(clip.url);
    sound.onended = () => finish(false);
    sound.onerror = () => finish(true);
    sound.play().catch(() => finish(true));
  }

  return {
    enqueue(clip: Clip) {
      if (current) waiting.push(clip);
      else start(clip);
    },
    playing: () => current?.id ?? null,
    subscribe(listen: () => void) {
      listeners.add(listen);
      return () => {
        listeners.delete(listen);
      };
    },
  };
}

export const player = createPlayer((url) => new Audio(url));
