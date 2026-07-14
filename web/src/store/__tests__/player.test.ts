import { createPinia, setActivePinia } from "pinia";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Music } from "@/types/api";
import { usePlayerStore } from "../player";

const makeTrack = (id: number, overrides: Partial<Music> = {}): Music => ({
  id,
  album: "",
  album_id: null,
  artist: `Artist ${id}`,
  created_at: "2026-01-01T00:00:00Z",
  duration: 180,
  genre: "",
  img: "",
  intro: "",
  issuing_date: "2026-01-01T00:00:00Z",
  path: `/api/v1/musics/${id}/stream`,
  title: `Track ${id}`,
  track_number: id,
  type: "single",
  updated_at: "2026-01-01T00:00:00Z",
  user_id: 1,
  year: 2026,
  ...overrides,
});

const makeAudio = (duration = 180) => {
  let source = "";
  const audio = {
    currentTime: 0,
    duration,
    load: vi.fn(),
    pause: vi.fn(),
    play: vi.fn().mockResolvedValue(undefined),
    removeAttribute: vi.fn((name: string) => {
      if (name === "src") source = "";
    }),
    getAttribute: vi.fn((name: string) => (name === "src" ? source || null : null)),
    volume: 1,
    get src() {
      return source;
    },
    set src(value: string) {
      source = value;
    },
  };
  return audio as unknown as HTMLAudioElement;
};

describe("usePlayerStore", () => {
  beforeEach(() => {
    localStorage.clear();
    setActivePinia(createPinia());
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("ignores corrupted persisted playback state", () => {
    localStorage.setItem("player-state:v1", "{broken");

    const store = usePlayerStore();

    expect(store.queue).toEqual([]);
    expect(store.currentTrack).toBeNull();
    expect(store.currentTime).toBe(0);
  });

  it("normalizes a playback context and starts the selected track", async () => {
    const store = usePlayerStore();
    const audio = makeAudio();
    const first = makeTrack(1);
    const selected = makeTrack(2);
    store.attachAudio(audio);

    const played = await store.playTrack(selected, [first, selected, selected, makeTrack(3, { path: "" })]);

    expect(played).toBe(true);
    expect(store.queue.map((track) => track.id)).toEqual([1, 2]);
    expect(store.currentTrack?.id).toBe(2);
    expect(audio.src).toBe(selected.path);
    expect(audio.load).toHaveBeenCalledOnce();
    expect(audio.play).toHaveBeenCalledOnce();
    expect(store.recentTracks[0]?.track.id).toBe(2);
  });

  it("rejects tracks without a playable path", async () => {
    const store = usePlayerStore();
    store.attachAudio(makeAudio());

    await expect(store.playTrack(makeTrack(1, { path: "" }))).resolves.toBe(false);

    expect(store.queue).toEqual([]);
  });

  it("clamps seeking to the loaded duration and persists progress", async () => {
    const store = usePlayerStore();
    const audio = makeAudio(120);
    store.attachAudio(audio);
    await store.playTrack(makeTrack(1));
    store.handleLoadedMetadata();

    store.seek(999);

    expect(audio.currentTime).toBe(120);
    expect(store.currentTime).toBe(120);
    expect(store.progressPercent).toBe(100);
    expect(localStorage.getItem("player-state:v1")).toContain('"currentTime":120');
  });

  it("advances to the next queued track when playback ends", async () => {
    const store = usePlayerStore();
    const audio = makeAudio();
    const first = makeTrack(1);
    const second = makeTrack(2);
    store.attachAudio(audio);
    await store.playTrack(first, [first, second]);

    await store.handleEnded();

    expect(store.currentTrack?.id).toBe(2);
    expect(audio.src).toBe(second.path);
    expect(audio.play).toHaveBeenCalledTimes(2);
  });

  it("deduplicates rapid playback errors but reports later failures", () => {
    vi.useFakeTimers();
    vi.setSystemTime(1000);
    const store = usePlayerStore();

    store.handleError();
    store.handleError();
    expect(store.playbackErrorId).toBe(1);

    vi.advanceTimersByTime(501);
    store.handleError();
    expect(store.playbackErrorId).toBe(2);
  });

  it("clamps volume and restores the last audible level after mute", () => {
    const store = usePlayerStore();
    const audio = makeAudio();
    store.attachAudio(audio);

    store.setVolume(2);
    expect(store.volume).toBe(1);
    expect(audio.volume).toBe(1);

    store.toggleMute();
    expect(store.volume).toBe(0);
    store.toggleMute();
    expect(store.volume).toBe(1);
  });
});
