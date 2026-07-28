<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import { useI18n } from "vue-i18n";
import { useRoute, useRouter } from "vue-router";
import MusicCover from "@/components/music/MusicCover.vue";
import MusicTrackTable from "@/components/music/MusicTrackTable.vue";
import { usePaginatedFetch } from "@/composables/usePaginatedFetch";
import type { AlbumSummary, ArtistSummary, Music } from "@/types/api";
import request from "@/utils/request";

const route = useRoute();
const router = useRouter();
const { t } = useI18n();
const artist = ref<ArtistSummary>();
const artistLoading = ref(false);
const artistKey = computed(() => String(route.params.key || ""));
const trackParams = computed(() => ({ artist_key: artistKey.value }));
const albumParams = computed(() => ({ artist_key: artistKey.value }));

const {
  items: tracks,
  loading: tracksLoading,
  total: trackTotal,
  currentPage: trackPage,
  pageSize: trackPageSize,
  fetch: fetchTracks,
} = usePaginatedFetch<Music>("/musics", { initialPageSize: 20, extraParams: trackParams });
const {
  items: albums,
  loading: albumsLoading,
  total: albumTotal,
  currentPage: albumPage,
  pageSize: albumPageSize,
  fetch: fetchAlbums,
} = usePaginatedFetch<AlbumSummary>("/albums", { initialPageSize: 12, extraParams: albumParams });

const routePage = (value: unknown) => {
  const parsed = Number.parseInt(typeof value === "string" ? value : "", 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : 1;
};

const loadArtist = async () => {
  artistLoading.value = true;
  try {
    artist.value = (await request.get<ArtistSummary>(`/artists/${encodeURIComponent(artistKey.value)}`)).data;
	} catch {
		ElMessage.error(t("library.artist_load_failed"));
  } finally {
    artistLoading.value = false;
  }
};

watch(
  artistKey,
  () => {
    artist.value = undefined;
    void loadArtist();
  },
  { immediate: true },
);
watch(
  [artistKey, () => route.query.track_page],
  () => {
    trackPage.value = routePage(route.query.track_page);
    void fetchTracks();
  },
  { immediate: true },
);
watch(
  [artistKey, () => route.query.album_page],
  () => {
    albumPage.value = routePage(route.query.album_page);
    void fetchAlbums();
  },
  { immediate: true },
);

const updatePage = (name: "track_page" | "album_page", page: number) => {
  const query = { ...route.query };
  if (page > 1) query[name] = String(page);
  else delete query[name];
  void router.push({ name: "ArtistDetail", params: { key: artistKey.value }, query });
};
</script>

<template>
  <section class="page-section">
    <el-skeleton v-if="artistLoading && !artist" :rows="5" animated />
    <template v-else-if="artist">
      <header class="entity-hero">
        <div class="hero-cover"><MusicCover :src="artist.cover_url" show-placeholder-label /></div>
        <div class="hero-copy">
          <el-text type="info">{{ $t("library.artist") }}</el-text>
          <h1>{{ artist.name || $t("music.unknown_artist") }}</h1>
          <p>{{ $t("library.artist_counts", { tracks: artist.track_count, albums: artist.album_count }) }}</p>
          <el-tag v-if="artist.musicbrainz_artist_id" effect="plain">
            MusicBrainz · {{ artist.musicbrainz_artist_id }}
          </el-tag>
        </div>
      </header>

      <section class="detail-section">
        <h2>{{ $t("library.albums") }}</h2>
        <div v-if="albumsLoading" class="loading-wrap"><el-skeleton :rows="3" animated /></div>
        <el-empty v-else-if="albums.length === 0" :description="$t('library.no_albums_for_artist')" />
        <div v-else class="album-strip">
          <router-link
            v-for="album in albums"
            :key="album.key"
            class="album-link"
            :to="{ name: 'AlbumDetail', params: { key: album.key } }"
          >
            <MusicCover :src="album.cover_url" show-placeholder-label />
            <strong>{{ album.title }}</strong>
            <span>{{ album.year || $t("library.year_unknown") }}</span>
          </router-link>
        </div>
        <el-pagination
          v-if="albumTotal > albumPageSize"
          layout="prev, pager, next"
          :current-page="albumPage"
          :page-size="albumPageSize"
          :total="albumTotal"
          @current-change="(page: number) => updatePage('album_page', page)"
        />
      </section>

      <section class="detail-section">
        <h2>{{ $t("library.tracks") }}</h2>
        <div v-if="tracksLoading" class="loading-wrap"><el-skeleton :rows="5" animated /></div>
        <el-empty v-else-if="tracks.length === 0" :description="$t('common.no_music_found')" />
        <MusicTrackTable v-else :tracks="tracks" :playback-context="tracks" show-album />
        <el-pagination
          v-if="trackTotal > trackPageSize"
          layout="prev, pager, next"
          :current-page="trackPage"
          :page-size="trackPageSize"
          :total="trackTotal"
          @current-change="(page: number) => updatePage('track_page', page)"
        />
      </section>
    </template>
  </section>
</template>

<style scoped lang="scss">
.entity-hero {
  display: grid;
  grid-template-columns: minmax(180px, 280px) 1fr;
  align-items: end;
  gap: $spacing-2xl;
  margin-bottom: $spacing-3xl;
}

.hero-cover {
  aspect-ratio: 1;
  overflow: hidden;
  border-radius: $radius-xl;
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.14);
}

.hero-copy {
  h1 {
    margin: $spacing-xs 0 $spacing-sm;
    font-size: clamp(2rem, 6vw, 4rem);
  }

  p {
    color: var(--text-secondary);
  }
}

.detail-section {
  margin-top: $spacing-2xl;

  h2 {
    margin-bottom: $spacing-lg;
  }

  > .el-pagination {
    justify-content: center;
    margin-top: $spacing-lg;
  }
}

.album-strip {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
  gap: $spacing-lg;
}

.album-link {
  display: grid;
  gap: $spacing-xs;
  color: var(--text-primary);
  text-decoration: none;

  :deep(.music-cover) {
    aspect-ratio: 1;
    overflow: hidden;
    border-radius: $radius-md;
  }

  span {
    color: var(--text-secondary);
    font-size: $fs-sm;
  }
}

@include mobile {
  .entity-hero {
    grid-template-columns: 1fr;
    align-items: start;
  }

  .hero-cover {
    width: min(75vw, 280px);
  }
}
</style>
