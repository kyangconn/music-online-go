<script setup lang="ts">
import {
  Close,
  DArrowLeft,
  DArrowRight,
  List,
  Mute,
  VideoPause,
  VideoPlay,
} from "@element-plus/icons-vue";
import { ElMessage } from "element-plus";
import { storeToRefs } from "pinia";
import { computed, onMounted, onUnmounted, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { useRouter } from "vue-router";
import MusicCover from "@/components/music/MusicCover.vue";
import { useMediaSession } from "@/composables/useMediaSession";
import { usePlayerStore } from "@/store/player";
import { formatPlaybackTime } from "@/utils/playback";

const router = useRouter();
const { t } = useI18n();
const playerStore = usePlayerStore();
const audioRef = ref<HTMLAudioElement>();
const {
  queue,
  currentIndex,
  currentTrack,
  hasPrevious,
  hasNext,
  isPlaying,
  currentTime,
  duration,
  volume,
  playbackErrorId,
} = storeToRefs(playerStore);

const mediaSessionTrack = computed(() => {
  const track = currentTrack.value;
  if (!track) return null;

  return {
    title: track.title,
    artist: track.artist,
    album: track.album_id ? t("common.album_track") : t("common.single"),
    artworkUrl: track.img || track.cover_url,
  };
});

onMounted(() => playerStore.attachAudio(audioRef.value));
onUnmounted(() => playerStore.attachAudio());

useMediaSession({
  audioRef,
  track: mediaSessionTrack,
  isPlaying,
  currentTime,
  duration,
  onPrevious: () => void playerStore.previous(),
  onNext: () => void playerStore.next(),
  onPlaybackError: playerStore.handleError,
});

watch(playbackErrorId, (errorId) => {
  if (!errorId) return;
  ElMessage.error(t("player.playback_failed"));
});

const openCurrentTrack = () => {
  if (currentTrack.value) void router.push(`/music/${currentTrack.value.id}`);
};

const handleSeek = (value: number | number[]) => {
  if (typeof value === "number") playerStore.seek(value);
};

const handleVolume = (value: number | number[]) => {
  if (typeof value === "number") playerStore.setVolume(value);
};

const selectQueueTrack = (index: number) => {
  void playerStore.selectQueueIndex(index);
};

const handleAudioError = () => {
  playerStore.handleError();
};
</script>

<template>
  <audio
    ref="audioRef"
    preload="metadata"
    hidden
    @timeupdate="playerStore.handleTimeUpdate"
    @loadedmetadata="playerStore.handleLoadedMetadata"
    @play="playerStore.handlePlay"
    @pause="playerStore.handlePause"
    @ended="playerStore.handleEnded"
    @error="handleAudioError"
  />

  <aside v-if="currentTrack" class="global-player" :aria-label="$t('player.now_playing')">
    <div class="player-content">
      <button class="track-summary" type="button" @click="openCurrentTrack">
        <span class="track-cover">
          <MusicCover :src="currentTrack.img || currentTrack.cover_url" />
        </span>
        <span class="track-copy">
          <strong class="track-title">{{ currentTrack.title }}</strong>
          <span class="track-artist">{{ currentTrack.artist }}</span>
        </span>
      </button>

      <div class="transport-controls">
        <el-tooltip :content="$t('player.previous')" placement="top">
          <el-button
            circle
            text
            :icon="DArrowLeft"
            :disabled="!hasPrevious"
            :aria-label="$t('player.previous')"
            @click="playerStore.previous"
          />
        </el-tooltip>
        <el-tooltip :content="isPlaying ? $t('player.pause') : $t('player.play')" placement="top">
          <el-button
            class="primary-control"
            circle
            type="primary"
            :icon="isPlaying ? VideoPause : VideoPlay"
            :aria-label="isPlaying ? $t('player.pause') : $t('player.play')"
            @click="playerStore.togglePlayback"
          />
        </el-tooltip>
        <el-tooltip :content="$t('player.next')" placement="top">
          <el-button
            circle
            text
            :icon="DArrowRight"
            :disabled="!hasNext"
            :aria-label="$t('player.next')"
            @click="playerStore.next"
          />
        </el-tooltip>
      </div>

      <div class="timeline">
        <span class="time-label">{{ formatPlaybackTime(currentTime) }}</span>
        <el-slider
          :model-value="currentTime"
          :max="Math.max(duration, 1)"
          :show-tooltip="false"
          :aria-label="$t('player.seek')"
          @input="handleSeek"
        />
        <span class="time-label">{{ formatPlaybackTime(duration) }}</span>
      </div>

      <div class="player-actions">
        <div class="volume-control">
          <el-tooltip :content="volume > 0 ? $t('player.mute') : $t('player.unmute')" placement="top">
            <el-button
              circle
              text
              :icon="Mute"
              :aria-label="volume > 0 ? $t('player.mute') : $t('player.unmute')"
              @click="playerStore.toggleMute"
            />
          </el-tooltip>
          <el-slider
            :model-value="volume"
            :min="0"
            :max="1"
            :step="0.05"
            :show-tooltip="false"
            :aria-label="$t('player.volume')"
            @input="handleVolume"
          />
        </div>

        <el-dropdown trigger="click" placement="top-end">
          <span class="queue-trigger" :title="$t('player.queue')" :aria-label="$t('player.queue')">
            <el-icon><List /></el-icon>
            <span>{{ queue.length }}</span>
          </span>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item disabled>{{ $t("player.queue_count", { count: queue.length }) }}</el-dropdown-item>
              <el-dropdown-item v-for="(track, index) in queue" :key="track.id" @click="selectQueueTrack(index)">
                <el-icon v-if="index === currentIndex"><VideoPlay /></el-icon>
                <span class="queue-track">{{ track.title }} - {{ track.artist }}</span>
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>

        <el-tooltip :content="$t('player.close')" placement="top">
          <el-button circle text :icon="Close" :aria-label="$t('player.close')" @click="playerStore.clear" />
        </el-tooltip>
      </div>
    </div>
  </aside>
</template>

<style scoped lang="scss">
.global-player {
  position: fixed;
  left: 50%;
  bottom: $spacing-lg;
  z-index: 2000;
  width: min(1120px, calc(100% - 32px));
  transform: translateX(-50%);
  border: 1px solid var(--border-color);
  border-radius: $radius-md;
  background: var(--bg-card);
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.18);
  container: global-player / inline-size;
}

.player-content {
  display: grid;
  grid-template-columns: minmax(160px, 240px) auto minmax(180px, 1fr) auto;
  grid-template-areas: "track transport timeline actions";
  align-items: center;
  gap: $spacing-md;
  min-height: 72px;
  padding: $spacing-sm $spacing-md;
}

.track-summary {
  grid-area: track;
  display: grid;
  grid-template-columns: 52px minmax(0, 1fr);
  align-items: center;
  gap: $spacing-sm;
  min-width: 0;
  padding: 0;
  border: 0;
  background: transparent;
  color: inherit;
  text-align: left;
  cursor: pointer;
}

.track-cover {
  width: 52px;
  aspect-ratio: 1;
  overflow: hidden;
  border-radius: $radius-sm;
}

.track-copy {
  min-width: 0;
}

.track-title,
.track-artist {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.track-title {
  color: var(--text-primary);
  font-size: $fs-base;
}

.track-artist {
  margin-top: $spacing-xs;
  color: var(--text-secondary);
  font-size: $fs-sm;
}

.transport-controls {
  grid-area: transport;
  display: grid;
  grid-template-columns: repeat(3, 32px);
  align-items: center;
  gap: $spacing-xs;
}

.primary-control {
  width: 32px;
  height: 32px;
}

.timeline {
  grid-area: timeline;
  display: grid;
  grid-template-columns: 40px minmax(120px, 1fr) 40px;
  align-items: center;
  gap: $spacing-sm;
  min-width: 0;
}

.time-label {
  color: var(--text-secondary);
  font-size: $fs-sm;
  text-align: center;
}

.player-actions {
  grid-area: actions;
  display: flex;
  align-items: center;
  gap: $spacing-sm;
}

.volume-control {
  display: grid;
  grid-template-columns: 32px 88px;
  align-items: center;
  gap: $spacing-xs;
  color: var(--text-secondary);
}

.queue-track {
  display: block;
  max-width: 260px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.queue-trigger {
  display: grid;
  grid-template-columns: 16px auto;
  align-items: center;
  gap: 2px;
  min-width: 32px;
  min-height: 32px;
  padding: 0 $spacing-xs;
  color: var(--text-secondary);
  cursor: pointer;
}

@container global-player (max-width: 760px) {
  .player-content {
    grid-template-columns: minmax(0, 1fr) auto;
    grid-template-areas:
      "track transport"
      "timeline actions";
  }

  .volume-control {
    display: none;
  }

  .player-actions {
    justify-content: flex-end;
  }
}

@container global-player (max-width: 520px) {
  .player-content {
    grid-template-columns: minmax(0, 1fr) auto;
    gap: $spacing-xs $spacing-sm;
  }

  .track-summary {
    grid-template-columns: 44px minmax(0, 1fr);
  }

  .track-cover {
    width: 44px;
  }

  .timeline {
    grid-template-columns: minmax(100px, 1fr) 36px;
  }

  .timeline .time-label:first-child {
    display: none;
  }
}

@media (max-width: 640px) {
  .global-player {
    bottom: 0;
    width: 100%;
    border-right: 0;
    border-bottom: 0;
    border-left: 0;
    border-radius: $radius-md $radius-md 0 0;
  }
}
</style>
