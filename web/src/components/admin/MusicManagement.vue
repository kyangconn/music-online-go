<script setup lang="ts">
import { Delete, Search, View } from "@element-plus/icons-vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { useRouter } from "vue-router";
import type { Music, PaginatedData } from "@/types/api";
import MusicCover from "@/components/music/MusicCover.vue";
import request from "@/utils/request";

const router = useRouter();
const { t } = useI18n();
const loading = ref(false);
const musics = ref<Music[]>([]);
const query = ref("");
const currentPage = ref(1);
const pageSize = ref(10);
const total = ref(0);

const fetchMusics = async () => {
  loading.value = true;
  try {
    const res = await request.get<PaginatedData<Music>>("/musics", {
      params: {
        q: query.value,
        page: currentPage.value,
        page_size: pageSize.value,
      },
    });
    musics.value = res.data.items || [];
    total.value = res.data.total;
  } catch (_e) {
    ElMessage.error(t("admin.load_music_failed"));
  } finally {
    loading.value = false;
  }
};

const handleSearch = () => {
  currentPage.value = 1;
  fetchMusics();
};

const deleteMusic = async (music: Music) => {
  try {
    await ElMessageBox.confirm(
      t("admin.delete_music_confirm", { title: music.title }),
      t("admin.delete_music"),
      {
        confirmButtonText: t("common.delete"),
        cancelButtonText: t("common.cancel"),
        type: "warning",
      },
    );
    await request.delete(`/users/admin/musics/${music.id}`);
    ElMessage.success(t("admin.music_deleted"));
    fetchMusics();
  } catch (error) {
    if (error !== "cancel") ElMessage.error(t("admin.music_delete_failed"));
  }
};

onMounted(fetchMusics);
</script>

<template>
  <section class="admin-panel">
    <div class="admin-toolbar">
      <el-input
        v-model="query"
        :placeholder="$t('admin.search_music')"
        clearable
        class="search-input"
        @keyup.enter="handleSearch"
        @clear="handleSearch"
      >
        <template #prefix>
          <el-icon><Search /></el-icon>
        </template>
      </el-input>
      <el-button type="primary" :icon="Search" @click="handleSearch">{{ $t("admin.search") }}</el-button>
    </div>

    <el-table v-loading="loading" :data="musics" size="small" class="admin-table">
      <el-table-column width="72">
        <template #default="{ row }">
          <div class="cover-thumb">
            <MusicCover :src="row.img || row.cover_url" />
          </div>
        </template>
      </el-table-column>
      <el-table-column prop="title" :label="$t('add.music_title')" min-width="180" show-overflow-tooltip />
      <el-table-column prop="artist" :label="$t('add.music_artist')" min-width="160" show-overflow-tooltip />
      <el-table-column prop="user_id" :label="$t('admin.uploader_id')" width="100" />
      <el-table-column prop="like_count" :label="$t('common.likes')" width="90" />
      <el-table-column :label="$t('admin.actions')" width="150" fixed="right">
        <template #default="{ row }">
          <el-button :icon="View" circle size="small" @click="router.push(`/music/${row.id}`)" />
          <el-button :icon="Delete" circle size="small" type="danger" @click="deleteMusic(row)" />
        </template>
      </el-table-column>
    </el-table>

    <div class="admin-pagination">
      <el-pagination
        v-model:current-page="currentPage"
        background
        layout="prev, pager, next"
        :page-size="pageSize"
        :total="total"
        @current-change="fetchMusics"
      />
    </div>
  </section>
</template>

<style scoped lang="scss">
.admin-panel {
  width: 100%;
}

.admin-toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: $spacing-sm;
  margin-bottom: $spacing-lg;
}

.search-input {
  max-width: 320px;
}

.cover-thumb {
  width: 44px;
  aspect-ratio: 1;
  border-radius: $radius-sm;
  overflow: hidden;
}

.admin-table {
  width: 100%;
}

.admin-pagination {
  @include flex-center;
  margin-top: $spacing-lg;
}
</style>
