<script setup lang="ts">
import { ArrowLeft, RefreshLeft, StarFilled, VideoPause, VideoPlay } from "@element-plus/icons-vue";
import { ElMessage } from "element-plus";
import { computed, onMounted, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { useRoute, useRouter } from "vue-router";
import type { Music, MusicFilterOptions, MusicType } from "@/types/api";
import MusicCover from "@/components/music/MusicCover.vue";
import RecentPlayback from "@/components/player/RecentPlayback.vue";
import { usePaginatedFetch } from "@/composables/usePaginatedFetch";
import { usePlayerStore } from "@/store/player";
import { useUserStore } from "@/store/user";
import request from "@/utils/request";

const route = useRoute();
const router = useRouter();
const { t } = useI18n();
const playerStore = usePlayerStore();
const userStore = useUserStore();
const searchQuery = ref("");
const artistFilter = ref("");
const yearFilter = ref<number>();
const typeFilter = ref<MusicType>();
const likedFilter = ref(false);
const filterOptions = ref<MusicFilterOptions>({ artists: [], years: [], types: [] });

const queryString = (value: unknown) => (typeof value === "string" ? value : "");
const activeFilterCount = computed(
  () => Number(Boolean(artistFilter.value)) + Number(Boolean(yearFilter.value)) + Number(Boolean(typeFilter.value)) + Number(likedFilter.value),
);

const requestParams = computed(() => ({
  q: searchQuery.value || undefined,
  artist: artistFilter.value || undefined,
  year: yearFilter.value,
  type: typeFilter.value,
  liked: likedFilter.value || undefined,
}));

const { items: musicList, loading, total, currentPage, pageSize, resetAndFetch, goToPage } =
  usePaginatedFetch<Music>("/musics", {
    initialPageSize: 12,
    errorMessageKey: "common.load_failed",
    extraParams: requestParams,
  });

watch(
  () => route.query,
  (query) => {
    searchQuery.value = queryString(query.q);
    artistFilter.value = queryString(query.artist);
    const parsedYear = Number.parseInt(queryString(query.year), 10);
    yearFilter.value = Number.isFinite(parsedYear) && parsedYear > 0 ? parsedYear : undefined;
    const routeType = queryString(query.type);
    typeFilter.value = routeType === "single" || routeType === "album" ? routeType : undefined;
    likedFilter.value = userStore.isLoggedIn && queryString(query.liked) === "true";
    resetAndFetch();
  },
  { immediate: true },
);

const buildRouteQuery = () => ({
  ...(searchQuery.value ? { q: searchQuery.value } : {}),
  ...(artistFilter.value ? { artist: artistFilter.value } : {}),
  ...(yearFilter.value ? { year: String(yearFilter.value) } : {}),
  ...(typeFilter.value ? { type: typeFilter.value } : {}),
  ...(likedFilter.value ? { liked: "true" } : {}),
});

const applyFilters = () => {
  void router.push({ name: "Home", query: buildRouteQuery() });
};

const resetFilters = () => {
  artistFilter.value = "";
  yearFilter.value = undefined;
  typeFilter.value = undefined;
  likedFilter.value = false;
  applyFilters();
};

const clearSearch = () => {
  searchQuery.value = "";
  applyFilters();
};

const handlePlayback = (music: Music) => {
  void playerStore.toggleTrack(music, musicList.value);
};

const loadFilterOptions = async () => {
  try {
    const response = await request.get<MusicFilterOptions>("/musics/filters");
    filterOptions.value = response.data;
  } catch {
    ElMessage.error(t("music.filter_options_failed"));
  }
};

onMounted(() => void loadFilterOptions());
</script>

<template>
  <div class="home-container">
    <div class="banner">
      <h1>{{ $t("nav.discover") }}</h1>
      <p>{{ $t("nav.discover_desc") }}</p>
    </div>

    <RecentPlayback />

    <div class="music-section">
      <div class="section-heading">
        <h2 v-if="searchQuery">{{ $t("common.search_results_for", { query: searchQuery }) }}</h2>
        <h2 v-else>{{ $t("nav.recommended") }}</h2>
        <el-button v-if="searchQuery" plain :icon="ArrowLeft" @click="clearSearch">
          {{ $t("common.back") }}
        </el-button>
      </div>

      <div class="filter-bar">
        <el-select
          v-model="artistFilter"
          class="filter-control artist-filter"
          :placeholder="$t('music.filter_artist')"
          clearable
          filterable
          allow-create
          default-first-option
          @change="applyFilters"
        >
          <el-option v-for="artist in filterOptions.artists" :key="artist" :label="artist" :value="artist" />
        </el-select>
        <el-select
          v-model="yearFilter"
          class="filter-control"
          :placeholder="$t('music.filter_year')"
          clearable
          @change="applyFilters"
        >
          <el-option v-for="year in filterOptions.years" :key="year" :label="String(year)" :value="year" />
        </el-select>
        <el-select
          v-model="typeFilter"
          class="filter-control"
          :placeholder="$t('music.filter_type')"
          clearable
          @change="applyFilters"
        >
          <el-option :label="$t('common.single')" value="single" />
          <el-option :label="$t('common.album')" value="album" />
        </el-select>
        <el-checkbox v-if="userStore.isLoggedIn" v-model="likedFilter" @change="applyFilters">
          {{ $t("music.liked_only") }}
        </el-checkbox>
        <el-tooltip v-if="activeFilterCount" :content="$t('music.reset_filters')" placement="top">
          <el-button circle plain :icon="RefreshLeft" :aria-label="$t('music.reset_filters')" @click="resetFilters" />
        </el-tooltip>
      </div>

      <div v-if="loading" class="loading-container">
        <el-skeleton :rows="3" animated />
      </div>

      <div v-else-if="musicList.length === 0" class="empty-state">
        <el-empty :description="$t('common.no_music_found')" />
      </div>

      <div v-else class="music-grid-tight">
        <el-card
          v-for="music in musicList"
          :key="music.id"
          class="music-card"
          shadow="hover"
        >
          <div class="cover-image">
            <router-link class="cover-link" :to="`/music/${music.id}`">
              <MusicCover :src="music.img || music.cover_url" />
            </router-link>
            <div v-if="music.path" class="play-overlay">
              <el-tooltip
                :content="
                  playerStore.currentTrack?.id === music.id && playerStore.isPlaying
                    ? $t('player.pause')
                    : $t('player.play')
                "
                placement="top"
              >
                <el-button
                  type="primary"
                  circle
                  :icon="
                    playerStore.currentTrack?.id === music.id && playerStore.isPlaying ? VideoPause : VideoPlay
                  "
                  :aria-label="
                    playerStore.currentTrack?.id === music.id && playerStore.isPlaying
                      ? $t('player.pause')
                      : $t('player.play')
                  "
                  size="large"
                  @click="handlePlayback(music)"
                />
              </el-tooltip>
            </div>
          </div>
          <div class="music-info">
            <h3 class="music-title" :title="music.title">
              <router-link :to="`/music/${music.id}`">{{ music.title }}</router-link>
            </h3>
            <p class="music-artist">{{ music.artist }}</p>
            <div class="music-meta">
              <el-icon class="like-icon"><StarFilled /></el-icon>
              <span class="like-count">{{ music.like_count ?? 0 }}</span>
            </div>
          </div>
        </el-card>
      </div>

      <div class="pagination-container" v-if="total > pageSize">
        <el-pagination
          background
          layout="prev, pager, next"
          :total="total"
          :page-size="pageSize"
          v-model:current-page="currentPage"
          @current-change="goToPage"
        />
      </div>
    </div>
  </div>
</template>

<style scoped lang="scss">
.banner {
  background: linear-gradient(135deg, var(--secondary-color), var(--primary-color));
  color: white;
  padding: 3rem 2rem;
  border-radius: $radius-md;
  margin-bottom: 2rem;
  text-align: center;

  h1 {
    margin: 0 0 1rem;
    font-size: $fs-hero;
  }
  p {
    margin: 0;
    opacity: 0.8;
    font-size: $fs-xl;
  }
}

.music-grid-tight {
  container: music-grid / inline-size;
}

.music-card {
  transition: transform $transition-slow;
  border: none;
  background: var(--bg-white);

  :deep(.el-card__body) {
    padding: 0;
  }

  &:hover {
    transform: translateY(-5px);
  }
}

.section-heading {
  @include flex-between;
  gap: $spacing-md;
  margin-bottom: $spacing-lg;

  h2 {
    margin: 0;
  }
}

.filter-bar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: $spacing-sm;
  margin-bottom: $spacing-lg;
  padding-bottom: $spacing-md;
  border-bottom: 1px solid var(--border-color);
}

.filter-control {
  width: 140px;
}

.artist-filter {
  width: 200px;
}

.cover-image {
  @include aspect-ratio;
  :deep(.music-cover) {
    @include aspect-ratio-inner;
  }
}

.cover-link {
  display: block;
  width: 100%;
  height: 100%;
}

.play-overlay {
  @include overlay-hover;
}

.music-card:hover .play-overlay {
  opacity: 1;
}

.music-card:focus-within .play-overlay {
  opacity: 1;
}

.music-info {
  padding: $spacing-md;
}

.music-title {
  margin: 0 0 5px;
  font-size: $fs-md;
  @include text-ellipsis;
  color: var(--text-dark);

  a {
    color: inherit;
    text-decoration: none;
  }
}

.music-artist {
  margin: 0;
  font-size: $fs-sm;
  color: var(--text-light);
}

.music-meta {
  margin-top: 6px;
  @include inline-flex($spacing-xs);
  font-size: $fs-sm;
  color: var(--text-light);
}

.like-icon {
  color: $color-gold;
}

.pagination-container {
  @include flex-center;
  margin-top: 2rem;
}

@supports (grid-template-rows: subgrid) {
  .music-card {
    display: grid;
    grid-row: span 4;
    grid-template-rows: subgrid;
    overflow: hidden;

    :deep(.el-card__body) {
      display: contents;
    }
  }

  .music-info {
    display: contents;
  }

  .music-title {
    margin: $spacing-md $spacing-md 5px;
  }

  .music-artist {
    margin: 0 $spacing-md;
  }

  .music-meta {
    margin: 6px $spacing-md $spacing-md;
  }
}

@container music-grid (max-width: 520px) {
  .music-title {
    margin-right: $spacing-sm;
    margin-left: $spacing-sm;
    font-size: $fs-sm;
  }

  .music-artist,
  .music-meta {
    margin-right: $spacing-sm;
    margin-left: $spacing-sm;
  }
}

@media (hover: none) {
  .play-overlay {
    align-items: flex-end;
    justify-content: flex-end;
    box-sizing: border-box;
    padding: $spacing-sm;
    background: transparent;
    opacity: 1;
  }
}

@include mobile {
  .section-heading {
    align-items: flex-start;
    flex-direction: column;
  }

  .filter-control,
  .artist-filter {
    width: 100%;
  }
}
</style>
