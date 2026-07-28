<script setup lang="ts">
import { Search } from "@element-plus/icons-vue";
import { computed, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import MusicCover from "@/components/music/MusicCover.vue";
import { usePaginatedFetch } from "@/composables/usePaginatedFetch";
import type { ArtistSummary } from "@/types/api";

const route = useRoute();
const router = useRouter();
const query = ref("");
const requestParams = computed(() => ({ q: query.value || undefined }));
const { items, loading, total, currentPage, pageSize, fetch } = usePaginatedFetch<ArtistSummary>("/artists", {
  initialPageSize: 24,
  extraParams: requestParams,
});

const routeString = (value: unknown) => (typeof value === "string" ? value : "");
watch(
  () => route.query,
  (value) => {
    query.value = routeString(value.q);
    const page = Number.parseInt(routeString(value.page), 10);
    currentPage.value = Number.isFinite(page) && page > 0 ? page : 1;
    void fetch();
  },
  { immediate: true },
);

const applySearch = () => {
  void router.push({ name: "Artists", query: query.value ? { q: query.value } : {} });
};

const changePage = (page: number) => {
  void router.push({ name: "Artists", query: { ...(query.value ? { q: query.value } : {}), page: String(page) } });
};
</script>

<template>
  <section class="page-section library-page">
    <header class="library-heading">
      <div>
        <h1>{{ $t("library.artists") }}</h1>
        <p>{{ $t("library.artists_desc") }}</p>
      </div>
      <el-input
        v-model="query"
        class="library-search"
        clearable
        :prefix-icon="Search"
        :placeholder="$t('library.search_artists')"
        @keyup.enter="applySearch"
        @clear="applySearch"
      />
    </header>

    <div v-if="loading" class="loading-wrap"><el-skeleton :rows="5" animated /></div>
    <el-empty v-else-if="items.length === 0" :description="$t('library.no_artists')" />
    <div v-else class="music-grid-tight">
      <router-link
        v-for="artist in items"
        :key="artist.key"
        class="entity-link"
        :to="{ name: 'ArtistDetail', params: { key: artist.key } }"
      >
        <el-card class="entity-card" shadow="hover">
          <div class="entity-cover"><MusicCover :src="artist.cover_url" show-placeholder-label /></div>
          <div class="entity-copy">
            <h2>{{ artist.name || $t("music.unknown_artist") }}</h2>
            <p>{{ $t("library.artist_counts", { tracks: artist.track_count, albums: artist.album_count }) }}</p>
			<el-tag v-if="artist.musicbrainz_artist_id" size="small" effect="plain">
			  MusicBrainz · {{ artist.musicbrainz_artist_id.slice(0, 8) }}
			</el-tag>
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
      @current-change="changePage"
    />
  </section>
</template>

<style scoped lang="scss">
.library-heading {
  @include flex-between;
  gap: $spacing-xl;
  margin-bottom: $spacing-2xl;

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

  p {
    margin: 0 0 $spacing-sm;
    color: var(--text-secondary);
  }
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
}
</style>
