<script setup lang="ts">
import { ArrowDown, ArrowUp, Delete, VideoPause, VideoPlay } from "@element-plus/icons-vue";
import type { Music } from "@/types/api";
import { usePlayerStore } from "@/store/player";
import { formatDuration } from "@/utils/library";

const props = withDefaults(
  defineProps<{
    tracks: Music[];
    playbackContext?: Music[];
    showAlbum?: boolean;
    reorderable?: boolean;
    removable?: boolean;
  }>(),
  {
    playbackContext: undefined,
    showAlbum: false,
    reorderable: false,
    removable: false,
  },
);

const emit = defineEmits<{
  (event: "move", index: number, direction: -1 | 1): void;
  (event: "remove", music: Music): void;
}>();

const playerStore = usePlayerStore();
const toggleTrack = (music: Music) => {
  void playerStore.toggleTrack(music, props.playbackContext || props.tracks);
};
</script>

<template>
  <el-table :data="tracks" row-key="id" class="track-table">
    <el-table-column width="64" align="center">
      <template #default="{ row }">
        <el-button
          v-if="row.path"
          text
          circle
          :icon="playerStore.currentTrack?.id === row.id && playerStore.isPlaying ? VideoPause : VideoPlay"
          :aria-label="
            playerStore.currentTrack?.id === row.id && playerStore.isPlaying
              ? $t('player.pause')
              : $t('player.play')
          "
          @click="toggleTrack(row)"
        />
      </template>
    </el-table-column>
    <el-table-column :label="$t('music.track_number')" width="78" align="center">
      <template #default="{ row }">{{ row.track_number || "—" }}</template>
    </el-table-column>
    <el-table-column :label="$t('music.title')" min-width="190">
      <template #default="{ row }">
        <router-link class="track-title" :to="{ name: 'MusicDetail', params: { id: row.id } }">
          {{ row.title || $t("music.unknown_title") }}
        </router-link>
        <div class="track-artist">{{ row.artist || $t("music.unknown_artist") }}</div>
      </template>
    </el-table-column>
    <el-table-column v-if="showAlbum" :label="$t('common.album')" min-width="150" show-overflow-tooltip>
      <template #default="{ row }">{{ row.album || $t("music.unknown_album") }}</template>
    </el-table-column>
    <el-table-column :label="$t('common.duration')" width="90">
      <template #default="{ row }">{{ formatDuration(row.duration) }}</template>
    </el-table-column>
    <el-table-column v-if="reorderable || removable" :label="$t('common.actions')" width="150" align="right">
      <template #default="{ row, $index }">
        <el-button
          v-if="reorderable"
          text
          circle
          :icon="ArrowUp"
          :disabled="$index === 0"
          :aria-label="$t('playlist.move_up')"
          @click="emit('move', $index, -1)"
        />
        <el-button
          v-if="reorderable"
          text
          circle
          :icon="ArrowDown"
          :disabled="$index === tracks.length - 1"
          :aria-label="$t('playlist.move_down')"
          @click="emit('move', $index, 1)"
        />
        <el-button
          v-if="removable"
          text
          circle
          type="danger"
          :icon="Delete"
          :aria-label="$t('playlist.remove_track')"
          @click="emit('remove', row)"
        />
      </template>
    </el-table-column>
  </el-table>
</template>

<style scoped lang="scss">
.track-table {
  width: 100%;
}

.track-title {
  color: var(--text-primary);
  font-weight: $fw-semibold;
  text-decoration: none;

  &:hover {
    color: var(--accent-color);
  }
}

.track-artist {
  margin-top: 2px;
  color: var(--text-secondary);
  font-size: $fs-sm;
}

@include mobile {
  :deep(.el-table__cell) {
    padding-right: $spacing-xs;
    padding-left: $spacing-xs;
  }
}
</style>
