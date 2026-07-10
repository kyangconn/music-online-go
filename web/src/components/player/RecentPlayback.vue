<script setup lang="ts">
import { Delete, VideoPlay } from "@element-plus/icons-vue";
import { storeToRefs } from "pinia";
import { useRouter } from "vue-router";
import MusicCover from "@/components/music/MusicCover.vue";
import type { RecentTrackEntry } from "@/store/player";
import { usePlayerStore } from "@/store/player";
import { formatPlaybackTime } from "@/utils/playback";

const router = useRouter();
const playerStore = usePlayerStore();
const { recentTracks } = storeToRefs(playerStore);

const openDetail = (entry: RecentTrackEntry) => {
  void router.push(`/music/${entry.track.id}`);
};

const resume = (entry: RecentTrackEntry) => {
  void playerStore.resumeRecent(entry);
};
</script>

<template>
  <section v-if="recentTracks.length" class="recent-playback">
    <div class="recent-heading">
      <h2>{{ $t("player.recent") }}</h2>
      <el-tooltip :content="$t('player.clear_recent')" placement="top">
        <el-button
          circle
          text
          :icon="Delete"
          :aria-label="$t('player.clear_recent')"
          @click="playerStore.clearRecent"
        />
      </el-tooltip>
    </div>

    <div class="recent-list">
      <article v-for="entry in recentTracks.slice(0, 6)" :key="entry.track.id" class="recent-item">
        <button class="recent-summary" type="button" @click="openDetail(entry)">
          <span class="recent-cover">
            <MusicCover :src="entry.track.img || entry.track.cover_url" />
          </span>
          <span class="recent-copy">
            <strong>{{ entry.track.title }}</strong>
            <span>{{ entry.track.artist }}</span>
          </span>
        </button>
        <span class="resume-position">
          {{ $t("player.continue_at", { time: formatPlaybackTime(entry.position) }) }}
        </span>
        <el-tooltip :content="$t('player.continue_playing')" placement="top">
          <el-button
            circle
            type="primary"
            plain
            :icon="VideoPlay"
            :aria-label="$t('player.continue_playing')"
            @click="resume(entry)"
          />
        </el-tooltip>
      </article>
    </div>
  </section>
</template>

<style scoped lang="scss">
.recent-playback {
  margin-bottom: $spacing-2xl;
}

.recent-heading {
  @include flex-between;
  margin-bottom: $spacing-md;

  h2 {
    margin: 0;
  }
}

.recent-list {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: $spacing-sm $spacing-lg;
}

.recent-item {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto 32px;
  align-items: center;
  gap: $spacing-sm;
  min-width: 0;
  padding: $spacing-sm;
  border-bottom: 1px solid var(--border-color);
}

.recent-summary {
  display: grid;
  grid-template-columns: 44px minmax(0, 1fr);
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

.recent-cover {
  width: 44px;
  aspect-ratio: 1;
  overflow: hidden;
  border-radius: $radius-sm;
}

.recent-copy {
  min-width: 0;

  strong,
  span {
    display: block;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  strong {
    color: var(--text-primary);
    font-size: $fs-sm;
  }

  span {
    margin-top: 2px;
    color: var(--text-secondary);
    font-size: $fs-sm;
  }
}

.resume-position {
  color: var(--text-secondary);
  font-size: $fs-sm;
  white-space: nowrap;
}

@media (max-width: 520px) {
  .recent-list {
    grid-template-columns: 1fr;
  }

  .resume-position {
    display: none;
  }
}
</style>
