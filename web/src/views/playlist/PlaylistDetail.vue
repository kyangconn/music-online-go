<script setup lang="ts">
import { Delete, Edit, List, Plus, Search, VideoPlay } from "@element-plus/icons-vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { computed, reactive, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { useRoute, useRouter } from "vue-router";
import MusicTrackTable from "@/components/music/MusicTrackTable.vue";
import { usePlayerStore } from "@/store/player";
import type { Music, PaginatedData, PlaylistDetail } from "@/types/api";
import request from "@/utils/request";

const route = useRoute();
const router = useRouter();
const { t } = useI18n();
const playerStore = usePlayerStore();
const playlist = ref<PlaylistDetail>();
const loading = ref(false);
const saving = ref(false);
const editVisible = ref(false);
const addVisible = ref(false);
const searchQuery = ref("");
const searchLoading = ref(false);
const searchResults = ref<Music[]>([]);
const editForm = reactive({ name: "", description: "" });
const playlistID = computed(() => Number(route.params.id));
const tracks = computed(() => playlist.value?.items.map((item) => item.music) || []);
const playableTracks = computed(() => tracks.value.filter((track) => Boolean(track.path)));
const existingIDs = computed(() => new Set(tracks.value.map((music) => music.id)));

const load = async () => {
  loading.value = true;
  try {
    playlist.value = (await request.get<PlaylistDetail>(`/playlists/${playlistID.value}`)).data;
  } catch {
    ElMessage.error(t("playlist.load_failed"));
  } finally {
    loading.value = false;
  }
};

watch(playlistID, () => void load(), { immediate: true });

const openEdit = () => {
  if (!playlist.value) return;
  editForm.name = playlist.value.name;
  editForm.description = playlist.value.description;
  editVisible.value = true;
};

const saveEdit = async () => {
  if (!playlist.value || !editForm.name.trim()) return;
  saving.value = true;
  try {
    playlist.value = (await request.patch<PlaylistDetail>(`/playlists/${playlist.value.id}`, editForm)).data;
    editVisible.value = false;
    ElMessage.success(t("playlist.saved"));
  } catch {
    ElMessage.error(t("playlist.save_failed"));
  } finally {
    saving.value = false;
  }
};

const deletePlaylist = async () => {
  if (!playlist.value) return;
  try {
    await ElMessageBox.confirm(t("playlist.delete_confirm", { name: playlist.value.name }), t("common.delete"), {
      type: "warning",
    });
    await request.delete(`/playlists/${playlist.value.id}`);
    ElMessage.success(t("playlist.deleted"));
    await router.push({ name: "Playlists" });
  } catch (error) {
    if (error !== "cancel" && error !== "close") ElMessage.error(t("playlist.delete_failed"));
  }
};

const searchMusic = async () => {
  searchLoading.value = true;
  try {
    const response = await request.get<PaginatedData<Music>>("/musics", {
      params: { q: searchQuery.value || undefined, page: 1, page_size: 20 },
    });
    searchResults.value = response.data.items || [];
  } catch {
    ElMessage.error(t("common.load_failed"));
  } finally {
    searchLoading.value = false;
  }
};

const openAdd = () => {
  searchQuery.value = "";
  searchResults.value = [];
  addVisible.value = true;
  void searchMusic();
};

const addTrack = async (music: Music) => {
  if (!playlist.value) return;
  try {
    playlist.value = (
      await request.post<PlaylistDetail>(`/playlists/${playlist.value.id}/items`, { music_id: music.id })
    ).data;
    ElMessage.success(t("playlist.track_added"));
  } catch {
    ElMessage.error(t("playlist.track_add_failed"));
  }
};

const removeTrack = async (music: Music) => {
  if (!playlist.value) return;
  try {
    playlist.value = (
      await request.delete<PlaylistDetail>(`/playlists/${playlist.value.id}/items/${music.id}`)
    ).data;
    ElMessage.success(t("playlist.track_removed"));
  } catch {
    ElMessage.error(t("playlist.track_remove_failed"));
  }
};

const moveTrack = async (index: number, direction: -1 | 1) => {
  if (!playlist.value) return;
  const target = index + direction;
  const ids = tracks.value.map((music) => music.id);
  if (target < 0 || target >= ids.length) return;
  [ids[index], ids[target]] = [ids[target]!, ids[index]!];
  try {
    playlist.value = (
      await request.put<PlaylistDetail>(`/playlists/${playlist.value.id}/items/order`, { music_ids: ids })
    ).data;
  } catch {
    ElMessage.error(t("playlist.reorder_failed"));
  }
};

const playPlaylist = () => void playerStore.playCollection(tracks.value);
const enqueuePlaylist = () => {
  const count = playerStore.enqueueTracks(tracks.value);
  if (count) ElMessage.success(t("library.queued_tracks", { count }));
  else ElMessage.info(playableTracks.value.length ? t("library.already_queued") : t("library.no_playable_tracks"));
};
</script>

<template>
  <section class="page-section">
    <el-skeleton v-if="loading && !playlist" :rows="7" animated />
    <template v-else-if="playlist">
      <header class="playlist-hero">
        <div>
          <el-text type="info">{{ $t("playlist.private_playlist") }}</el-text>
          <h1>{{ playlist.name }}</h1>
          <p>{{ playlist.description || $t("playlist.no_description") }}</p>
          <span>{{ $t("playlist.track_count", { count: playlist.item_count }) }}</span>
        </div>
        <div class="hero-actions">
          <el-button type="primary" :icon="VideoPlay" :disabled="playableTracks.length === 0" @click="playPlaylist">
            {{ $t("playlist.play") }}
          </el-button>
          <el-button :icon="List" :disabled="playableTracks.length === 0" @click="enqueuePlaylist">
            {{ $t("library.add_to_queue") }}
          </el-button>
          <el-button :icon="Plus" @click="openAdd">{{ $t("playlist.add_tracks") }}</el-button>
          <el-button :icon="Edit" @click="openEdit">{{ $t("common.edit") }}</el-button>
          <el-button type="danger" plain :icon="Delete" @click="deletePlaylist">{{ $t("common.delete") }}</el-button>
        </div>
      </header>

      <el-empty v-if="tracks.length === 0" :description="$t('playlist.no_tracks')">
        <el-button type="primary" :icon="Plus" @click="openAdd">{{ $t("playlist.add_tracks") }}</el-button>
      </el-empty>
      <MusicTrackTable
        v-else
        :tracks="tracks"
        :playback-context="tracks"
        show-album
        reorderable
        removable
        @move="moveTrack"
        @remove="removeTrack"
      />
    </template>

    <el-dialog v-model="editVisible" :title="$t('playlist.edit')" width="min(520px, 92vw)">
      <el-form label-position="top">
        <el-form-item :label="$t('playlist.name')" required>
          <el-input v-model="editForm.name" maxlength="120" show-word-limit />
        </el-form-item>
        <el-form-item :label="$t('playlist.form_description')">
          <el-input v-model="editForm.description" type="textarea" :rows="4" maxlength="1000" show-word-limit />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">{{ $t("common.cancel") }}</el-button>
        <el-button type="primary" :loading="saving" :disabled="!editForm.name.trim()" @click="saveEdit">
          {{ $t("common.save") }}
        </el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="addVisible" :title="$t('playlist.add_tracks')" width="min(760px, 94vw)">
      <el-input
        v-model="searchQuery"
        clearable
        :prefix-icon="Search"
        :placeholder="$t('common.search')"
        @keyup.enter="searchMusic"
        @clear="searchMusic"
      >
        <template #append>
          <el-button :icon="Search" :loading="searchLoading" @click="searchMusic" />
        </template>
      </el-input>
      <el-table v-loading="searchLoading" :data="searchResults" class="search-results">
        <el-table-column prop="title" :label="$t('music.title')" min-width="180" />
        <el-table-column prop="artist" :label="$t('music.artists')" min-width="140" />
        <el-table-column prop="album" :label="$t('common.album')" min-width="140">
          <template #default="{ row }">{{ row.album || $t("music.unknown_album") }}</template>
        </el-table-column>
        <el-table-column width="110" align="right">
          <template #default="{ row }">
            <el-button
              size="small"
              :disabled="existingIDs.has(row.id)"
              @click="addTrack(row)"
            >
              {{ existingIDs.has(row.id) ? $t("playlist.added") : $t("playlist.add") }}
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-dialog>
  </section>
</template>

<style scoped lang="scss">
.playlist-hero {
  @include flex-between;
  align-items: flex-end;
  gap: $spacing-xl;
  margin-bottom: $spacing-2xl;

  h1 {
    margin: $spacing-xs 0;
    font-size: clamp(2rem, 5vw, 3.5rem);
  }

  p,
  span {
    color: var(--text-secondary);
  }
}

.hero-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: $spacing-sm;
}

.search-results {
  margin-top: $spacing-md;
}

@include mobile {
  .playlist-hero {
    align-items: stretch;
    flex-direction: column;
  }

  .hero-actions {
    justify-content: flex-start;
  }
}
</style>
