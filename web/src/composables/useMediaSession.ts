import { onMounted, onUnmounted, watch, type ComputedRef, type Ref } from "vue";

export interface MediaSessionTrack {
  title: string;
  artist: string;
  album?: string;
  artworkUrl?: string;
}

interface UseMediaSessionOptions {
  audioRef: Ref<HTMLAudioElement | undefined>;
  track: ComputedRef<MediaSessionTrack | null>;
  isPlaying: Ref<boolean>;
  currentTime: Ref<number>;
  duration: Ref<number>;
  onPrevious?: () => void;
  onNext?: () => void;
  onPlaybackError?: () => void;
}

const isSupported = () => typeof navigator !== "undefined" && "mediaSession" in navigator;

/**
 * Connects one HTML audio element to the browser and operating system media controls.
 * It intentionally owns no playback state so it can later be reused by a global player.
 */
export function useMediaSession({
  audioRef,
  track,
  isPlaying,
  currentTime,
  duration,
  onPrevious,
  onNext,
  onPlaybackError,
}: UseMediaSessionOptions) {
  const updatePosition = () => {
    if (!isSupported() || !Number.isFinite(duration.value) || duration.value <= 0) return;

    try {
      navigator.mediaSession.setPositionState({
        duration: duration.value,
        playbackRate: audioRef.value?.playbackRate || 1,
        position: Math.min(Math.max(currentTime.value, 0), duration.value),
      });
    } catch {
      // Some browsers expose Media Session but do not support position updates.
    }
  };

  const seekBy = (offset: number) => {
    const audio = audioRef.value;
    if (!audio) return;
    const target = Math.min(Math.max(audio.currentTime + offset, 0), Number.isFinite(audio.duration) ? audio.duration : Infinity);
    audio.currentTime = target;
  };

  const play = () => {
    const audio = audioRef.value;
    if (audio) {
      void audio.play().catch(() => {
        onPlaybackError?.();
      });
    }
  };

  const pause = () => {
    audioRef.value?.pause();
  };

  const setActionHandler = (action: MediaSessionAction, handler: MediaSessionActionHandler | null) => {
    try {
      navigator.mediaSession.setActionHandler(action, handler);
    } catch {
      // Support for individual Media Session actions varies by browser.
    }
  };

  const setupActionHandlers = () => {
    if (!isSupported()) return;

    setActionHandler("play", play);
    setActionHandler("pause", pause);
    setActionHandler("seekbackward", (details) => seekBy(-(details.seekOffset || 10)));
    setActionHandler("seekforward", (details) => seekBy(details.seekOffset || 10));
    setActionHandler("seekto", (details) => {
      if (typeof details.seekTime === "number") {
        const audio = audioRef.value;
        if (audio) audio.currentTime = details.seekTime;
      }
    });
    if (onPrevious) setActionHandler("previoustrack", onPrevious);
    if (onNext) setActionHandler("nexttrack", onNext);
  };

  const clear = () => {
    if (!isSupported()) return;

    const session = navigator.mediaSession;
    session.metadata = null;
    session.playbackState = "none";
    const actions: MediaSessionAction[] = ["play", "pause", "seekbackward", "seekforward", "seekto"];
    if (onPrevious) actions.push("previoustrack");
    if (onNext) actions.push("nexttrack");
    for (const action of actions) {
      setActionHandler(action, null);
    }
  };

  onMounted(() => {
    if (!isSupported()) return;

    setupActionHandlers();
  });

  onUnmounted(clear);

  watch(
    track,
    (value) => {
      if (!isSupported()) return;

      navigator.mediaSession.metadata = value
        ? new MediaMetadata({
            title: value.title,
            artist: value.artist,
            album: value.album,
            artwork: value.artworkUrl ? [{ src: value.artworkUrl }] : [],
          })
        : null;
    },
    { immediate: true },
  );

  watch(isPlaying, (playing) => {
    if (isSupported()) navigator.mediaSession.playbackState = playing ? "playing" : "paused";
  });

  watch([currentTime, duration], updatePosition);
}
