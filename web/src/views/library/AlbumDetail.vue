<script setup lang="ts">
import { List, VideoPlay } from "@element-plus/icons-vue";
import { ElMessage } from "element-plus";
import { computed, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { useRoute } from "vue-router";
import MusicCover from "@/components/music/MusicCover.vue";
import MusicTrackTable from "@/components/music/MusicTrackTable.vue";
import { usePlayerStore } from "@/store/player";
import type { AlbumSummary, Music } from "@/types/api";
import { fetchMusicCollection, formatDuration } from "@/utils/library";
import request from "@/utils/request";

const route = useRoute();
const { t } = useI18n();
const playerStore = usePlayerStore();
const album = ref<AlbumSummary>();
const tracks = ref<Music[]>([]);
const loading = ref(false);
const albumKey = computed(() => String(route.params.key || ""));
const playableTracks = computed(() => tracks.value.filter((track) => Boolean(track.path)));

const discs = computed(() => {
  const groups = new Map<number, Music[]>();
  for (const track of tracks.value) {
    const disc = track.disc_number > 0 ? track.disc_number : 1;
    const values = groups.get(disc) || [];
    values.push(track);
    groups.set(disc, values);
  }
  return [...groups.entries()].sort(([left], [right]) => left - right);
});

const load = async () => {
  loading.value = true;
  try {
    const [albumResponse, music] = await Promise.all([
      request.get<AlbumSummary>(`/albums/${encodeURIComponent(albumKey.value)}`),
      fetchMusicCollection({ album_key: albumKey.value }),
    ]);
    album.value = albumResponse.data;
    tracks.value = music;
  } catch {
    ElMessage.error(t("library.album_load_failed"));
  } finally {
    loading.value = false;
  }
};

watch(albumKey, () => void load(), { immediate: true });

const playAlbum = () => {
  void playerStore.playCollection(tracks.value);
};

const enqueueAlbum = () => {
  const count = playerStore.enqueueTracks(tracks.value);
  if (count) ElMessage.success(t("library.queued_tracks", { count }));
  else ElMessage.info(playableTracks.value.length ? t("library.already_queued") : t("library.no_playable_tracks"));
};
</script>

<template>
  <section class="page-section">
    <el-skeleton v-if="loading && !album" :rows="7" animated />
    <template v-else-if="album">
      <header class="album-hero">
        <div class="hero-cover"><MusicCover :src="album.cover_url" show-placeholder-label /></div>
        <div class="hero-copy">
          <el-text type="info">{{ $t("common.album") }}</el-text>
          <h1>{{ album.title || $t("music.unknown_album") }}</h1>
          <router-link
            v-if="album.album_artist_key"
            class="artist-link"
            :to="{ name: 'ArtistDetail', params: { key: album.album_artist_key } }"
          >
            {{ album.album_artist || $t("music.unknown_artist") }}
          </router-link>
          <p v-else>{{ album.album_artist || $t("music.unknown_artist") }}</p>
          <div class="album-meta">
            <span>{{ album.year || $t("library.year_unknown") }}</span>
            <span>·</span>
            <span>{{ $t("library.track_count", { count: album.track_count }) }}</span>
            <span>·</span>
            <span>{{ formatDuration(album.total_duration) }}</span>
            <el-tag v-if="album.disc_count > 1" size="small" effect="plain">
              {{ $t("library.disc_count", { count: album.disc_count }) }}
            </el-tag>
            <el-tag v-if="album.musicbrainz_release_id || album.musicbrainz_release_group_id" size="small" effect="plain">
              MusicBrainz · {{ album.musicbrainz_release_id || album.musicbrainz_release_group_id }}
            </el-tag>
          </div>
          <div class="album-actions">
            <el-button type="primary" :icon="VideoPlay" :disabled="playableTracks.length === 0" @click="playAlbum">
              {{ $t("library.play_album") }}
            </el-button>
            <el-button :icon="List" :disabled="playableTracks.length === 0" @click="enqueueAlbum">
              {{ $t("library.add_to_queue") }}
            </el-button>
          </div>
        </div>
      </header>

      <el-alert
        v-if="album.track_count > tracks.length"
        type="warning"
        :closable="false"
        :title="$t('library.collection_truncated', { count: tracks.length })"
      />

      <el-empty v-if="tracks.length === 0" :description="$t('library.album_has_no_tracks')" />
	  <template v-else>
		<section v-for="[disc, discTracks] in discs" :key="disc" class="disc-section">
		  <h2 v-if="discs.length > 1">{{ $t("library.disc", { number: disc }) }}</h2>
		  <MusicTrackTable :tracks="discTracks" :playback-context="tracks" />
		</section>
	  </template>
    </template>
  </section>
</template>

<style scoped lang="scss">
.album-hero {
  display: grid;
  grid-template-columns: minmax(200px, 320px) 1fr;
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
    margin: $spacing-xs 0;
    font-size: clamp(2rem, 6vw, 4rem);
  }

  > p {
    margin: 0;
    color: var(--text-secondary);
    font-size: $fs-lg;
  }
}

.artist-link {
  color: var(--accent-color);
  font-size: $fs-lg;
  text-decoration: none;
}

.album-meta,
.album-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: $spacing-sm;
  margin-top: $spacing-md;
}

.album-meta {
  color: var(--text-secondary);
}

.disc-section {
  margin-top: $spacing-xl;

  h2 {
    margin-bottom: $spacing-md;
    font-size: $fs-lg;
  }
}

@include mobile {
  .album-hero {
    grid-template-columns: 1fr;
    align-items: start;
  }

  .hero-cover {
    width: min(75vw, 320px);
  }
}
</style>
