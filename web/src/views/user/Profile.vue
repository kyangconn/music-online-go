<script setup lang="ts">
import { Edit, Plus, VideoPause, VideoPlay } from "@element-plus/icons-vue";
import { onMounted } from "vue";
import { useRouter } from "vue-router";
import type { Music } from "@/types/api";
import MusicCover from "@/components/music/MusicCover.vue";
import { usePaginatedFetch } from "@/composables/usePaginatedFetch";
import { usePlayerStore } from "@/store/player";
import { useUserStore } from "@/store/user";

const router = useRouter();
const userStore = useUserStore();
const playerStore = usePlayerStore();
const { items: musicList, loading, fetch } = usePaginatedFetch<Music>("", {
  errorMessageKey: "common.load_failed",
});

const goUpload = () => {
  router.push("/music/add");
};

const handlePlayback = (music: Music) => {
  void playerStore.toggleTrack(music, musicList.value);
};

onMounted(() => {
  const userId = userStore.user?.id;
  if (userId) fetch(`/users/${userId}/musics`);
});
</script>

<template>
  <div class="page-section profile-container">
    <el-card class="content-card" shadow="never">
      <div class="user-header">
        <div class="user-info">
          <h2 class="username">{{ userStore.user?.username }}</h2>
          <p class="sub">{{ userStore.user?.email }}</p>
        </div>
      </div>

      <h3 class="section-title">{{ $t("common.uploaded_musics") }}</h3>
      <div v-if="loading">
        <el-skeleton :rows="3" animated />
      </div>
      <div v-else-if="musicList.length === 0" class="empty">
        <el-empty :description="$t('common.no_uploaded_musics')" />
      </div>
      <div v-else class="music-grid">
        <el-card v-for="m in musicList" :key="m.id" class="music-card" shadow="hover">
          <div class="cover">
            <MusicCover :src="m.img || m.cover_url" />
          </div>
          <div class="meta">
            <h4 class="title">{{ m.title }}</h4>
            <p class="artist">{{ m.artist }}</p>
            <p class="likes">{{ $t("common.likes") }}: {{ m.like_count ?? 0 }}</p>
            <div class="card-actions">
              <el-tooltip
                v-if="m.path"
                :content="
                  playerStore.currentTrack?.id === m.id && playerStore.isPlaying
                    ? $t('player.pause')
                    : $t('player.play')
                "
                placement="top"
              >
                <el-button
                  circle
                  size="small"
                  :icon="playerStore.currentTrack?.id === m.id && playerStore.isPlaying ? VideoPause : VideoPlay"
                  :aria-label="
                    playerStore.currentTrack?.id === m.id && playerStore.isPlaying
                      ? $t('player.pause')
                      : $t('player.play')
                  "
                  @click="handlePlayback(m)"
                />
              </el-tooltip>
              <router-link :to="`/music/${m.id}`">
                <el-button size="small" type="primary">{{ $t("common.view") }}</el-button>
              </router-link>
              <el-button size="small" :icon="Edit" @click="router.push(`/music/${m.id}/edit`)">
                {{ $t("common.edit") }}
              </el-button>
            </div>
          </div>
        </el-card>
      </div>
    </el-card>
  </div>
  <el-button class="upload-fab" type="primary" circle size="large" :icon="Plus" @click="goUpload" />
</template>

<style scoped lang="scss">
.user-header {
  @include inline-flex($spacing-md);
  margin-bottom: $spacing-lg;
}
.user-info .username {
  margin: 0;
  color: var(--text-dark);
}
.sub {
  margin: 0;
  color: var(--text-light);
}
.section-title {
  margin: $spacing-lg 0;
}

.music-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: $spacing-lg;
}
.music-card {
  container: profile-music-card / inline-size;

  :deep(.el-card__body) {
    display: grid;
    grid-template-columns: 80px minmax(0, 1fr);
    align-items: start;
    gap: $spacing-md;
    width: 100%;
    box-sizing: border-box;
  }
}
.cover {
  width: 80px;
  height: 80px;
  border-radius: $radius-md;
  overflow: hidden;
}
.meta {
  min-width: 0;

  .title {
    margin: 0 0 6px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .artist {
    margin: 0 0 10px;
    color: var(--text-light);
  }
  .likes {
    margin: 0 0 6px;
    color: var(--text-light);
    font-size: $fs-sm;
  }
}

@container profile-music-card (max-width: 280px) {
  .music-card :deep(.el-card__body) {
    grid-template-columns: 64px minmax(0, 1fr);
    gap: $spacing-sm;
    padding: $spacing-sm;
  }

  .cover {
    width: 64px;
    height: 64px;
  }

  .card-actions {
    gap: $spacing-xs;
  }
}
.card-actions {
  display: flex;
  flex-wrap: wrap;
  gap: $spacing-xs;
}
.empty {
  margin-top: $spacing-md;
}

.upload-fab {
  position: fixed;
  right: $spacing-3xl;
  bottom: 100px;
  z-index: 1000;
  width: 56px;
  height: 56px;
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.15);
  transition:
    transform $transition-base,
    box-shadow $transition-base;
  &:hover {
    transform: translateY(-2px);
    box-shadow: 0 10px 28px rgba(0, 0, 0, 0.22);
  }
}
</style>
