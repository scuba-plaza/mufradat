export interface Clip {
  id: string;
  url: string;
  onError: () => void;
}

interface Sound {
  onended: ((event: Event) => unknown) | null;
  onerror: ((event: Event | string) => unknown) | null;
  onpause?: ((event: Event) => unknown) | null;
  play(): Promise<void>;
}

export function createPlayer(make: (url: string) => Sound) {
  let current: Clip | null = null;
  let waiting: Clip[] = [];
  const listeners = new Set<() => void>();
  const tell = () => listeners.forEach((listen) => listen());

  function start(clip: Clip) {
    current = clip;
    tell();
    let over = false;
    const finish = (failed: boolean) => {
      if (over) return;
      over = true;
      current = null;
      try {
        if (failed) clip.onError();
      } finally {
        const next = waiting.shift();
        if (next) start(next);
        else tell();
      }
    };
    try {
      const sound = make(clip.url);
      sound.onended = () => finish(false);
      sound.onpause = () => finish(false);
      sound.onerror = () => finish(true);
      sound.play().catch(() => finish(true));
    } catch {
      finish(true);
    }
  }

  return {
    enqueue(clip: Clip) {
      if (!current) return start(clip);
      waiting = [...waiting.filter((w) => w.id !== clip.id), clip];
    },
    forget(id: string) {
      waiting = waiting.filter((w) => w.id !== id);
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
