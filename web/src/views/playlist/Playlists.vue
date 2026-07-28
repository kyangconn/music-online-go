<script setup lang="ts">
import { Plus } from "@element-plus/icons-vue";
import { ElMessage } from "element-plus";
import { reactive, ref } from "vue";
import { useI18n } from "vue-i18n";
import { useRouter } from "vue-router";
import { usePaginatedFetch } from "@/composables/usePaginatedFetch";
import type { Playlist, PlaylistDetail } from "@/types/api";
import request from "@/utils/request";

const router = useRouter();
const { t } = useI18n();
const dialogVisible = ref(false);
const saving = ref(false);
const form = reactive({ name: "", description: "" });
const { items, loading, total, currentPage, pageSize, fetch, goToPage } = usePaginatedFetch<Playlist>("/playlists", {
  initialPageSize: 20,
});

void fetch();

const openCreate = () => {
  form.name = "";
  form.description = "";
  dialogVisible.value = true;
};

const createPlaylist = async () => {
  if (!form.name.trim()) return;
  saving.value = true;
  try {
    const response = await request.post<PlaylistDetail>("/playlists", form);
    dialogVisible.value = false;
    ElMessage.success(t("playlist.created"));
    await router.push({ name: "PlaylistDetail", params: { id: response.data.id } });
  } catch {
    ElMessage.error(t("playlist.create_failed"));
  } finally {
    saving.value = false;
  }
};
</script>

<template>
  <section class="page-section">
    <header class="playlist-heading">
      <div>
        <h1>{{ $t("playlist.title") }}</h1>
        <p>{{ $t("playlist.description") }}</p>
      </div>
      <el-button type="primary" :icon="Plus" @click="openCreate">{{ $t("playlist.create") }}</el-button>
    </header>

    <div v-if="loading" class="loading-wrap"><el-skeleton :rows="5" animated /></div>
    <el-empty v-else-if="items.length === 0" :description="$t('playlist.empty')">
      <el-button type="primary" @click="openCreate">{{ $t("playlist.create_first") }}</el-button>
    </el-empty>
    <div v-else class="card-grid">
      <router-link
        v-for="playlist in items"
        :key="playlist.id"
        class="playlist-link"
        :to="{ name: 'PlaylistDetail', params: { id: playlist.id } }"
      >
        <el-card class="playlist-card" shadow="hover">
          <h2>{{ playlist.name }}</h2>
          <p>{{ playlist.description || $t("playlist.no_description") }}</p>
          <span>{{ $t("playlist.track_count", { count: playlist.item_count }) }}</span>
        </el-card>
      </router-link>
    </div>

    <el-pagination
      v-if="total > pageSize"
      class="playlist-pagination"
      background
      layout="prev, pager, next"
      :current-page="currentPage"
      :page-size="pageSize"
      :total="total"
      @current-change="goToPage"
    />

    <el-dialog v-model="dialogVisible" :title="$t('playlist.create')" width="min(520px, 92vw)">
      <el-form label-position="top" @submit.prevent="createPlaylist">
        <el-form-item :label="$t('playlist.name')" required>
          <el-input v-model="form.name" maxlength="120" show-word-limit autofocus />
        </el-form-item>
        <el-form-item :label="$t('playlist.form_description')">
          <el-input v-model="form.description" type="textarea" :rows="4" maxlength="1000" show-word-limit />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">{{ $t("common.cancel") }}</el-button>
        <el-button type="primary" :loading="saving" :disabled="!form.name.trim()" @click="createPlaylist">
          {{ $t("playlist.create") }}
        </el-button>
      </template>
    </el-dialog>
  </section>
</template>

<style scoped lang="scss">
.playlist-heading {
  @include flex-between;
  gap: $spacing-lg;
  margin-bottom: $spacing-2xl;

  h1 {
    margin: 0 0 $spacing-xs;
  }

  p {
    margin: 0;
    color: var(--text-secondary);
  }
}

.playlist-link {
  color: inherit;
  text-decoration: none;
}

.playlist-card {
  height: 100%;

  h2 {
    margin: 0 0 $spacing-sm;
  }

  p {
    min-height: 2.8em;
    color: var(--text-secondary);
  }

  span {
    color: var(--text-secondary);
    font-size: $fs-sm;
  }
}

.playlist-pagination {
  justify-content: center;
  margin-top: $spacing-xl;
}

@include mobile {
  .playlist-heading {
    align-items: stretch;
    flex-direction: column;
  }
}
</style>
