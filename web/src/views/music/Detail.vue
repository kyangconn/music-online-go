<script setup lang="ts">
import { Edit, Star, StarFilled, VideoPause, VideoPlay } from "@element-plus/icons-vue";
import { ElMessage } from "element-plus";
import { storeToRefs } from "pinia";
import { computed, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { useRoute, useRouter } from "vue-router";
import type { Music } from "@/types/api";
import MusicCover from "@/components/music/MusicCover.vue";
import { useApiError } from "@/composables/useApiError";
import { usePlayerStore } from "@/store/player";
import { useUserStore } from "@/store/user";
import { formatPlaybackTime } from "@/utils/playback";
import request from "@/utils/request";

const route = useRoute();
const router = useRouter();
const { t } = useI18n();
const userStore = useUserStore();
const playerStore = usePlayerStore();
const { handleError } = useApiError();
const loading = ref(true);
const music = ref<Music | null>(null);
const metadataDuration = ref(0);
const isLiked = ref(false);
const likeCount = ref(0);
const { currentTrack, isPlaying, duration } = storeToRefs(playerStore);

const musicId = computed(() => {
  const value = route.params.id;
  return Array.isArray(value) ? (value[0] ?? "") : (value ?? "");
});

const isCurrentTrack = computed(() => currentTrack.value?.id === music.value?.id);
const displayedDuration = computed(() => {
  if (isCurrentTrack.value && duration.value) return duration.value;
  return metadataDuration.value;
});

const canManage = computed(() => {
  if (!music.value || !userStore.user) return false;
  return userStore.isAdmin || music.value.user_id === userStore.user.id;
});

const fetchDetail = async () => {
  const requestedId = musicId.value;
  if (!requestedId) {
    loading.value = false;
    return;
  }

  loading.value = true;
  metadataDuration.value = 0;
  try {
    const res = await request.get<Music>(`/musics/${requestedId}`);
    if (requestedId !== musicId.value) return;
    music.value = res.data;
    metadataDuration.value = res.data.duration || 0;
    isLiked.value = res.data.is_liked ?? false;
    likeCount.value = res.data.like_count ?? 0;
  } catch (e) {
    if (requestedId === musicId.value) handleError(e, t("music.load_detail_failed"));
  } finally {
    if (requestedId === musicId.value) loading.value = false;
  }
};

const handleMetadataLoaded = (event: Event) => {
  const audio = event.currentTarget as HTMLAudioElement;
  metadataDuration.value = Number.isFinite(audio.duration) ? audio.duration : 0;
};

const handlePlayback = () => {
  if (music.value?.path) void playerStore.toggleTrack(music.value);
};

const handleLike = async () => {
  if (!music.value) return;

  if (!userStore.isLoggedIn) {
    ElMessage.warning(t("common.please_login_first"));
    router.push("/login");
    return;
  }
  try {
    if (isLiked.value) {
      await request.delete(`/musics/${music.value.id}/like`);
      isLiked.value = false;
      likeCount.value = Math.max(0, likeCount.value - 1);
      ElMessage.success(t("common.unliked"));
    } else {
      await request.post(`/musics/${music.value.id}/like`);
      isLiked.value = true;
      likeCount.value += 1;
      ElMessage.success(t("common.liked"));
    }
  } catch (e) {
    handleError(e);
  }
};

watch(musicId, fetchDetail, { immediate: true });
</script>

<template>
  <div class="page-section detail-container">
    <el-card class="content-card" shadow="never">
      <div v-if="loading" class="loading">
        <el-skeleton :rows="6" animated />
      </div>
      <div v-else class="detail-content">
        <div class="detail-top">
          <div class="cover-section">
            <div class="cover">
              <MusicCover :src="music?.img || music?.cover_url" preview />
            </div>
          </div>
          <div class="info-section">
            <h1 class="music-title">{{ music?.title }}</h1>
            <p class="music-artist">{{ music?.artist }}</p>
            <el-descriptions :column="1" border>
              <el-descriptions-item :label="$t('common.type')">
                {{ music?.type === "album" ? $t("common.album") : $t("common.single") }}
              </el-descriptions-item>
              <el-descriptions-item v-if="music?.album" :label="$t('common.album')">{{ music.album }}</el-descriptions-item>
              <el-descriptions-item v-if="music?.year" :label="$t('music.year')">{{ music.year }}</el-descriptions-item>
              <el-descriptions-item v-if="music?.track_number" :label="$t('music.track_number')">
                {{ music.track_number }}
              </el-descriptions-item>
              <el-descriptions-item v-if="music?.genre" :label="$t('music.genre')">{{ music.genre }}</el-descriptions-item>
              <el-descriptions-item :label="$t('common.duration')">{{
                formatPlaybackTime(displayedDuration)
              }}</el-descriptions-item>
            </el-descriptions>
            <div class="likes-row">
              <el-button
                :type="isLiked ? 'warning' : 'default'"
                :icon="isLiked ? StarFilled : Star"
                @click="handleLike"
              >
                {{ likeCount }}
              </el-button>
            </div>
            <div class="actions">
              <el-button
                v-if="canManage"
                type="primary"
                :icon="Edit"
                @click="router.push(`/music/${music?.id}/edit`)"
              >
                {{ $t("common.edit") }}
              </el-button>
              <el-button @click="router.back()">{{ $t("common.back") }}</el-button>
            </div>
          </div>
        </div>

        <div v-if="music?.path" class="player-section">
          <audio :src="music.path" preload="metadata" hidden @loadedmetadata="handleMetadataLoaded" />
          <el-button
            type="primary"
            :icon="isCurrentTrack && isPlaying ? VideoPause : VideoPlay"
            @click="handlePlayback"
          >
            {{ isCurrentTrack && isPlaying ? $t("player.pause") : $t("player.play") }}
          </el-button>
        </div>
        <div v-else class="no-audio-hint">
          <el-alert :title="$t('common.no_audio_available')" type="info" :closable="false" show-icon />
        </div>
      </div>
    </el-card>
  </div>
</template>

<style scoped lang="scss">
.loading {
  padding: $spacing-xl 0;
}

.detail-top {
  display: flex;
  gap: $spacing-2xl;
  flex-wrap: wrap;
}

.cover-section {
  flex-shrink: 0;
}

.cover {
  width: 280px;
  aspect-ratio: 1;
  border-radius: $radius-md;
  overflow: hidden;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}

.info-section {
  flex: 1;
  min-width: 280px;
}

.music-title {
  margin: 0 0 $spacing-xs;
  font-size: $fs-3xl;
  color: var(--text-dark);
}
.music-artist {
  margin: 0 0 $spacing-lg;
  font-size: $fs-lg;
  color: var(--text-light);
}

.likes-row {
  margin-top: $spacing-md;
  @include inline-flex;
}
.actions {
  margin-top: $spacing-lg;
  display: flex;
  gap: $spacing-md;
}

.player-section {
  margin-top: $spacing-2xl;
  padding-top: $spacing-xl;
  border-top: 1px solid var(--border-color);
}
.no-audio-hint {
  margin-top: $spacing-xl;
}

@include mobile {
  .detail-top {
    flex-direction: column;
  }
  .cover {
    width: 100%;
    max-width: 280px;
    margin: 0 auto;
  }
}
</style>
