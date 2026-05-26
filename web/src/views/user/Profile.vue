<script setup lang="ts">
import { Plus } from "@element-plus/icons-vue"
import { ElMessage } from "element-plus"
import { ref, onMounted } from "vue"
import { useRouter } from "vue-router"
import type { Music, PaginatedData } from "@/types/api"
import { useUserStore } from "@/store/user"
import request from "@/utils/request"

const router = useRouter()
const userStore = useUserStore()
const loading = ref(true)
const musicList = ref<Music[]>([])

const fetchUserMusics = async () => {
  loading.value = true
  try {
    const userId = userStore.user?.id
    const res = await request.get<PaginatedData<Music>>(`/users/${userId}/musics`)
    musicList.value = res.data.items || []
  } catch (_e) {
    ElMessage.error("Failed to load user musics")
  } finally {
    loading.value = false
  }
}

const goUpload = () => {
  router.push("/music/add")
}

onMounted(fetchUserMusics)
</script>

<template>
  <div class="page-section profile-container">
    <el-card class="content-card" shadow="never">
      <div class="user-header">
        <div class="user-info">
          <h2 class="username">{{ userStore.user?.username }}</h2>
          <p class="sub">{{ userStore.user?.email }}</p>
        </div>
      </div>

      <h3 class="section-title">{{ $t("common.uploaded_musics") }}</h3>
      <div v-if="loading">
        <el-skeleton :rows="3" animated />
      </div>
      <div v-else-if="musicList.length === 0" class="empty">
        <el-empty :description="$t('common.no_uploaded_musics')" />
      </div>
      <div v-else class="music-grid">
        <el-card v-for="m in musicList" :key="m.id" class="music-card" shadow="hover">
          <div class="cover">
            <el-image :src="m.img || 'https://via.placeholder.com/160x160?text=Music'" fit="cover" />
          </div>
          <div class="meta">
            <h4 class="title">{{ m.title }}</h4>
            <p class="artist">{{ m.artist }}</p>
            <p class="likes">{{ $t("common.likes") }}: {{ m.like_count ?? 0 }}</p>
            <router-link :to="`/music/${m.id}`">
              <el-button size="small" type="primary">{{ $t("common.view") }}</el-button>
            </router-link>
          </div>
        </el-card>
      </div>
    </el-card>
  </div>
  <el-button class="upload-fab" type="primary" circle size="large" :icon="Plus" @click="goUpload" />
</template>

<style scoped lang="scss">
.user-header {
  @include inline-flex($spacing-md);
  margin-bottom: $spacing-lg;
}
.user-info .username {
  margin: 0;
  color: var(--text-dark);
}
.sub {
  margin: 0;
  color: var(--text-light);
}
.section-title {
  margin: $spacing-lg 0;
}

.music-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: $spacing-lg;
}
.music-card {
  display: flex;
  gap: $spacing-md;
}
.cover {
  width: 80px;
  height: 80px;
  border-radius: $radius-md;
  overflow: hidden;
}
.meta {
  .title {
    margin: 0 0 6px;
  }
  .artist {
    margin: 0 0 10px;
    color: var(--text-light);
  }
  .likes {
    margin: 0 0 6px;
    color: var(--text-light);
    font-size: $fs-sm;
  }
}
.empty {
  margin-top: $spacing-md;
}

.upload-fab {
  position: fixed;
  right: $spacing-3xl;
  bottom: 100px;
  z-index: 1000;
  width: 56px;
  height: 56px;
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.15);
  transition:
    transform $transition-base,
    box-shadow $transition-base;
  &:hover {
    transform: translateY(-2px);
    box-shadow: 0 10px 28px rgba(0, 0, 0, 0.22);
  }
}
</style>
