import { defineStore } from "pinia";
import { computed, ref } from "vue";
import type { Music } from "@/types/api";

const DEFAULT_VOLUME = 0.8;
const VOLUME_STORAGE_KEY = "player-volume";
const PLAYER_STATE_STORAGE_KEY = "player-state:v1";
const PLAYER_STATE_VERSION = 1;
const MAX_RECENT_TRACKS = 12;
const PERSIST_INTERVAL_MS = 5000;

export interface RecentTrackEntry {
  track: Music;
  position: number;
  playedAt: number;
}

interface PersistedPlayerState {
  version: number;
  queue: Music[];
  currentIndex: number;
  currentTime: number;
  recentTracks: RecentTrackEntry[];
}

const clamp = (value: number, min: number, max: number) => Math.min(Math.max(value, min), max);

const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === "object" && value !== null;

const isStoredMusic = (value: unknown): value is Music =>
  isRecord(value) &&
  typeof value.id === "number" &&
  typeof value.title === "string" &&
  typeof value.artist === "string" &&
  typeof value.path === "string";

const loadPlayerState = (): PersistedPlayerState | null => {
  if (typeof window === "undefined") return null;

  try {
    const raw = localStorage.getItem(PLAYER_STATE_STORAGE_KEY);
    if (!raw) return null;
    const parsed: unknown = JSON.parse(raw);
    if (!isRecord(parsed) || parsed.version !== PLAYER_STATE_VERSION || !Array.isArray(parsed.queue)) return null;

    const queue = parsed.queue.filter(isStoredMusic);
    const rawIndex = typeof parsed.currentIndex === "number" ? parsed.currentIndex : -1;
    const currentIndex = rawIndex >= 0 && rawIndex < queue.length ? rawIndex : queue.length > 0 ? 0 : -1;
    const currentTime = typeof parsed.currentTime === "number" && parsed.currentTime > 0 ? parsed.currentTime : 0;
    const recentTracks = Array.isArray(parsed.recentTracks)
      ? parsed.recentTracks.flatMap((entry): RecentTrackEntry[] => {
          if (!isRecord(entry) || !isStoredMusic(entry.track)) return [];
          return [
            {
              track: entry.track,
              position: typeof entry.position === "number" && entry.position > 0 ? entry.position : 0,
              playedAt: typeof entry.playedAt === "number" ? entry.playedAt : 0,
            },
          ];
        })
      : [];

    return { version: PLAYER_STATE_VERSION, queue, currentIndex, currentTime, recentTracks };
  } catch {
    return null;
  }
};

const loadVolume = () => {
  if (typeof window === "undefined") return DEFAULT_VOLUME;

  try {
    const storedValue = localStorage.getItem(VOLUME_STORAGE_KEY);
    if (storedValue === null) return DEFAULT_VOLUME;
    const value = Number(storedValue);
    return Number.isFinite(value) ? clamp(value, 0, 1) : DEFAULT_VOLUME;
  } catch {
    return DEFAULT_VOLUME;
  }
};

const normalizeQueue = (tracks: Music[]) => {
  const uniqueTracks = new Map<number, Music>();
  for (const track of tracks) {
    if (track.path) uniqueTracks.set(track.id, track);
  }
  return [...uniqueTracks.values()];
};

export const usePlayerStore = defineStore("player", () => {
  const restoredState = loadPlayerState();
  const queue = ref<Music[]>(restoredState?.queue ?? []);
  const currentIndex = ref(restoredState?.currentIndex ?? -1);
  const isPlaying = ref(false);
  const currentTime = ref(restoredState?.currentTime ?? 0);
  const duration = ref(0);
  const volume = ref(loadVolume());
  const playbackErrorId = ref(0);
  const recentTracks = ref<RecentTrackEntry[]>(restoredState?.recentTracks.slice(0, MAX_RECENT_TRACKS) ?? []);
  let lastAudibleVolume = volume.value > 0 ? volume.value : DEFAULT_VOLUME;
  let lastPlaybackErrorAt = 0;
  let lastPersistedAt = 0;
  let pendingSeek = currentTime.value;
  let audioElement: HTMLAudioElement | null = null;

  const currentTrack = computed(() => queue.value[currentIndex.value] ?? null);
  const hasTrack = computed(() => currentTrack.value !== null);
  const hasPrevious = computed(() => currentIndex.value > 0);
  const hasNext = computed(() => currentIndex.value >= 0 && currentIndex.value < queue.value.length - 1);
  const progressPercent = computed(() => {
    if (!duration.value) return 0;
    return (currentTime.value / duration.value) * 100;
  });

  const persistState = () => {
    try {
      const state: PersistedPlayerState = {
        version: PLAYER_STATE_VERSION,
        queue: queue.value,
        currentIndex: currentIndex.value,
        currentTime: currentTime.value,
        recentTracks: recentTracks.value,
      };
      localStorage.setItem(PLAYER_STATE_STORAGE_KEY, JSON.stringify(state));
      lastPersistedAt = Date.now();
    } catch {
      // Playback remains available when browser storage is unavailable.
    }
  };

  const updateRecent = (track: Music, position: number) => {
    const entry: RecentTrackEntry = {
      track,
      position: Math.max(position, 0),
      playedAt: Date.now(),
    };
    recentTracks.value = [entry, ...recentTracks.value.filter((item) => item.track.id !== track.id)].slice(
      0,
      MAX_RECENT_TRACKS,
    );
  };

  const attachAudio = (element?: HTMLAudioElement) => {
    audioElement = element ?? null;
    if (audioElement) {
      audioElement.volume = volume.value;
      const source = currentTrack.value?.path;
      if (source) {
        pendingSeek = currentTime.value;
        audioElement.src = source;
      }
    }
  };

  const play = async () => {
    const track = currentTrack.value;
    if (!audioElement || !track) return false;

    try {
      await audioElement.play();
      updateRecent(track, currentTime.value);
      persistState();
      return true;
    } catch {
      isPlaying.value = false;
      reportPlaybackError();
      return false;
    }
  };

  const pause = () => {
    audioElement?.pause();
  };

  const activateIndex = async (index: number, resumeAt = 0) => {
    if (index < 0 || index >= queue.value.length) return false;

    currentIndex.value = index;
    currentTime.value = Math.max(resumeAt, 0);
    duration.value = 0;
    pendingSeek = currentTime.value;
    const source = queue.value[index]?.path;
    if (audioElement && source && audioElement.getAttribute("src") !== source) {
      audioElement.src = source;
      audioElement.load();
    }
    const track = queue.value[index];
    if (track) updateRecent(track, currentTime.value);
    persistState();
    return play();
  };

  const playTrack = async (track: Music, context?: Music[]) => {
    if (!track.path) return false;

    if (context) {
      const nextQueue = normalizeQueue(context);
      const nextIndex = nextQueue.findIndex((item) => item.id === track.id);
      if (nextIndex < 0) return false;
      queue.value = nextQueue;
      return activateIndex(nextIndex);
    }

    const existingIndex = queue.value.findIndex((item) => item.id === track.id);
    if (existingIndex >= 0) return activateIndex(existingIndex);

    queue.value.push(track);
    return activateIndex(queue.value.length - 1);
  };

  const togglePlayback = async () => {
    if (isPlaying.value) {
      pause();
      return true;
    }
    return play();
  };

  const toggleTrack = (track: Music, context?: Music[]) => {
    if (currentTrack.value?.id === track.id) return togglePlayback();
    return playTrack(track, context);
  };

  const selectQueueIndex = async (index: number) => {
    if (index === currentIndex.value) return play();
    return activateIndex(index);
  };

  const resumeRecent = async (entry: RecentTrackEntry) => {
    const existingIndex = queue.value.findIndex((item) => item.id === entry.track.id);
    if (existingIndex >= 0) return activateIndex(existingIndex, entry.position);
    queue.value.push(entry.track);
    return activateIndex(queue.value.length - 1, entry.position);
  };

  const previous = async () => {
    if (!hasPrevious.value) return false;
    return activateIndex(currentIndex.value - 1);
  };

  const next = async () => {
    if (!hasNext.value) return false;
    return activateIndex(currentIndex.value + 1);
  };

  const seek = (seconds: number) => {
    if (!audioElement || !Number.isFinite(seconds)) return;
    audioElement.currentTime = clamp(seconds, 0, duration.value || 0);
    currentTime.value = audioElement.currentTime;
    const track = currentTrack.value;
    if (track) updateRecent(track, currentTime.value);
    persistState();
  };

  const setVolume = (value: number) => {
    volume.value = clamp(value, 0, 1);
    if (volume.value > 0) lastAudibleVolume = volume.value;
    if (audioElement) audioElement.volume = volume.value;

    try {
      localStorage.setItem(VOLUME_STORAGE_KEY, String(volume.value));
    } catch {
      // Playback still works when browser storage is unavailable.
    }
  };

  const toggleMute = () => {
    setVolume(volume.value > 0 ? 0 : lastAudibleVolume);
  };

  const handleTimeUpdate = () => {
    if (!audioElement) return;
    currentTime.value = audioElement.currentTime;
    if (Date.now() - lastPersistedAt < PERSIST_INTERVAL_MS) return;
    const track = currentTrack.value;
    if (track) updateRecent(track, currentTime.value);
    persistState();
  };

  const handleLoadedMetadata = () => {
    if (!audioElement || !Number.isFinite(audioElement.duration)) return;
    duration.value = audioElement.duration;
    if (pendingSeek > 0) {
      audioElement.currentTime = clamp(pendingSeek, 0, duration.value);
      currentTime.value = audioElement.currentTime;
      pendingSeek = 0;
    }
  };

  const handlePlay = () => {
    isPlaying.value = true;
  };

  const handlePause = () => {
    isPlaying.value = false;
    const track = currentTrack.value;
    if (track) updateRecent(track, currentTime.value);
    persistState();
  };

  const handleEnded = async () => {
    const track = currentTrack.value;
    if (track) updateRecent(track, 0);
    if (hasNext.value) {
      await next();
      return;
    }
    isPlaying.value = false;
    if (audioElement) audioElement.currentTime = 0;
    currentTime.value = 0;
    pendingSeek = 0;
    persistState();
  };

  const handleError = () => {
    isPlaying.value = false;
    reportPlaybackError();
  };

  function reportPlaybackError() {
    const now = Date.now();
    if (now - lastPlaybackErrorAt < 500) return;
    lastPlaybackErrorAt = now;
    playbackErrorId.value += 1;
  }

  const clear = () => {
    pause();
    audioElement?.removeAttribute("src");
    audioElement?.load();
    queue.value = [];
    currentIndex.value = -1;
    currentTime.value = 0;
    duration.value = 0;
    isPlaying.value = false;
    pendingSeek = 0;
    persistState();
  };

  const clearRecent = () => {
    recentTracks.value = [];
    persistState();
  };

  return {
    queue,
    currentIndex,
    currentTrack,
    hasTrack,
    hasPrevious,
    hasNext,
    isPlaying,
    currentTime,
    duration,
    volume,
    playbackErrorId,
    recentTracks,
    progressPercent,
    attachAudio,
    play,
    pause,
    playTrack,
    togglePlayback,
    toggleTrack,
    selectQueueIndex,
    resumeRecent,
    previous,
    next,
    seek,
    setVolume,
    toggleMute,
    handleTimeUpdate,
    handleLoadedMetadata,
    handlePlay,
    handlePause,
    handleEnded,
    handleError,
    clear,
    clearRecent,
  };
});
