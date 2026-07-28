<script setup lang="ts">
import { RefreshLeft, Search } from "@element-plus/icons-vue";
import { ElMessage } from "element-plus";
import { computed, onMounted, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { useRoute, useRouter } from "vue-router";
import MusicCover from "@/components/music/MusicCover.vue";
import { usePaginatedFetch } from "@/composables/usePaginatedFetch";
import type { AlbumSummary, MusicFilterOptions } from "@/types/api";
import request from "@/utils/request";

const route = useRoute();
const router = useRouter();
const { t } = useI18n();
const query = ref("");
const albumArtist = ref("");
const genre = ref("");
const year = ref<number>();
const filterOptions = ref<MusicFilterOptions>({
  artists: [],
  albums: [],
  album_artists: [],
  genres: [],
  years: [],
  types: [],
});

const requestParams = computed(() => ({
  q: query.value || undefined,
  album_artist: albumArtist.value || undefined,
  genre: genre.value || undefined,
  year: year.value,
}));
const { items, loading, total, currentPage, pageSize, fetch } = usePaginatedFetch<AlbumSummary>("/albums", {
  initialPageSize: 24,
  extraParams: requestParams,
});

const routeString = (value: unknown) => (typeof value === "string" ? value : "");
watch(
  () => route.query,
  (value) => {
    query.value = routeString(value.q);
    albumArtist.value = routeString(value.album_artist);
    genre.value = routeString(value.genre);
    const parsedYear = Number.parseInt(routeString(value.year), 10);
    year.value = Number.isFinite(parsedYear) && parsedYear > 0 ? parsedYear : undefined;
    const page = Number.parseInt(routeString(value.page), 10);
    currentPage.value = Number.isFinite(page) && page > 0 ? page : 1;
    void fetch();
  },
  { immediate: true },
);

const buildQuery = (page?: number) => ({
  ...(query.value ? { q: query.value } : {}),
  ...(albumArtist.value ? { album_artist: albumArtist.value } : {}),
  ...(genre.value ? { genre: genre.value } : {}),
  ...(year.value ? { year: String(year.value) } : {}),
  ...(page && page > 1 ? { page: String(page) } : {}),
});

const applyFilters = () => void router.push({ name: "Albums", query: buildQuery() });
const resetFilters = () => {
  query.value = "";
  albumArtist.value = "";
  genre.value = "";
  year.value = undefined;
  applyFilters();
};

const loadFilterOptions = async () => {
  try {
    filterOptions.value = (await request.get<MusicFilterOptions>("/musics/filters")).data;
  } catch {
    ElMessage.error(t("music.filter_options_failed"));
  }
};

onMounted(() => void loadFilterOptions());
</script>

<template>
  <section class="page-section">
    <header class="library-heading">
      <div>
        <h1>{{ $t("library.albums") }}</h1>
        <p>{{ $t("library.albums_desc") }}</p>
      </div>
      <el-input
        v-model="query"
        class="library-search"
        clearable
        :prefix-icon="Search"
        :placeholder="$t('library.search_albums')"
        @keyup.enter="applyFilters"
        @clear="applyFilters"
      />
    </header>

    <div class="filter-bar">
      <el-select
        v-model="albumArtist"
        filterable
        clearable
        allow-create
        default-first-option
        :placeholder="$t('music.filter_album_artist')"
        @change="applyFilters"
      >
        <el-option v-for="value in filterOptions.album_artists" :key="value" :label="value" :value="value" />
      </el-select>
      <el-select
        v-model="genre"
        filterable
        clearable
        allow-create
        default-first-option
        :placeholder="$t('music.filter_genre')"
        @change="applyFilters"
      >
        <el-option v-for="value in filterOptions.genres" :key="value" :label="value" :value="value" />
      </el-select>
      <el-select v-model="year" clearable :placeholder="$t('music.filter_year')" @change="applyFilters">
        <el-option v-for="value in filterOptions.years" :key="value" :label="String(value)" :value="value" />
      </el-select>
      <el-tooltip :content="$t('music.reset_filters')">
        <el-button circle plain :icon="RefreshLeft" :aria-label="$t('music.reset_filters')" @click="resetFilters" />
      </el-tooltip>
    </div>

    <div v-if="loading" class="loading-wrap"><el-skeleton :rows="5" animated /></div>
    <el-empty v-else-if="items.length === 0" :description="$t('library.no_albums')" />
    <div v-else class="music-grid-tight">
      <router-link
        v-for="album in items"
        :key="album.key"
        class="entity-link"
        :to="{ name: 'AlbumDetail', params: { key: album.key } }"
      >
        <el-card class="entity-card" shadow="hover">
          <div class="entity-cover"><MusicCover :src="album.cover_url" show-placeholder-label /></div>
          <div class="entity-copy">
            <h2>{{ album.title || $t("music.unknown_album") }}</h2>
            <p>{{ album.album_artist || $t("music.unknown_artist") }}</p>
            <div class="entity-meta">
              <span>{{ album.year || $t("library.year_unknown") }}</span>
              <span>·</span>
              <span>{{ $t("library.track_count", { count: album.track_count }) }}</span>
              <el-tag v-if="album.disc_count > 1" size="small" effect="plain">
                {{ $t("library.disc_count", { count: album.disc_count }) }}
              </el-tag>
			  <el-tag v-if="album.musicbrainz_release_id || album.musicbrainz_release_group_id" size="small" effect="plain">
				MusicBrainz · {{ (album.musicbrainz_release_id || album.musicbrainz_release_group_id).slice(0, 8) }}
			  </el-tag>
            </div>
          </div>
        </el-card>
      </router-link>
    </div>

    <el-pagination
      v-if="total > pageSize"
      class="library-pagination"
      background
      layout="prev, pager, next"
      :current-page="currentPage"
      :page-size="pageSize"
      :total="total"
      @current-change="(page: number) => router.push({ name: 'Albums', query: buildQuery(page) })"
    />
  </section>
</template>

<style scoped lang="scss">
.library-heading {
  @include flex-between;
  gap: $spacing-xl;
  margin-bottom: $spacing-xl;

  h1 {
    margin: 0 0 $spacing-xs;
  }

  p {
    margin: 0;
    color: var(--text-secondary);
  }
}

.library-search {
  width: min(340px, 100%);
}

.filter-bar {
  display: flex;
  flex-wrap: wrap;
  gap: $spacing-sm;
  margin-bottom: $spacing-xl;
}

.entity-link {
  color: inherit;
  text-decoration: none;
}

.entity-card {
  height: 100%;
  overflow: hidden;

  :deep(.el-card__body) {
    padding: 0;
  }
}

.entity-cover {
  aspect-ratio: 1;
}

.entity-copy {
  padding: $spacing-md;

  h2 {
    margin: 0 0 $spacing-xs;
    font-size: $fs-lg;
  }

  > p {
    margin: 0 0 $spacing-sm;
    color: var(--text-secondary);
  }
}

.entity-meta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: $spacing-xs;
  color: var(--text-secondary);
  font-size: $fs-sm;
}

.library-pagination {
  justify-content: center;
  margin-top: $spacing-xl;
}

@include mobile {
  .library-heading {
    align-items: stretch;
    flex-direction: column;
  }

  .filter-bar :deep(.el-select) {
    width: 100%;
  }
}
</style>
