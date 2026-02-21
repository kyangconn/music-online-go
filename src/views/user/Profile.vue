<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useUserStore } from '@/store/user'
import request from '@/utils/request'
import { ElMessage } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'

const router = useRouter()
const userStore = useUserStore()
const loading = ref(true)
const musicList = ref<any[]>([])

const fetchUserMusics = async () => {
  loading.value = true
  try {
    const userId = userStore.user?.id
    const res: any = await request.get(`/users/${userId}/musics`)
    musicList.value = res.data.items || []
  } catch (e) {
    ElMessage.error('Failed to load user musics')
  } finally {
    loading.value = false
  }
}

const goUpload = () => {
  router.push('/music/add')
}

onMounted(fetchUserMusics)
</script>

<template>
  <div class="profile-container">
    <el-card class="profile-card" shadow="never">
      <div class="user-header">
        <el-avatar :size="64" :src="userStore.user?.avatar_url" />
        <div class="user-info">
          <h2 class="username">{{ userStore.user?.username }}</h2>
          <p class="sub">{{ userStore.user?.email }}</p>
        </div>
      </div>

      <h3 class="section-title">Uploaded Musics</h3>
      <div v-if="loading">
        <el-skeleton :rows="3" animated />
      </div>
      <div v-else-if="musicList.length === 0" class="empty">
        <el-empty description="No uploaded musics" />
      </div>
      <div v-else class="music-grid">
        <el-card v-for="m in musicList" :key="m.id" class="music-card" shadow="hover">
          <div class="cover">
            <el-image :src="m.img || 'https://via.placeholder.com/160x160?text=Music'" fit="cover" />
          </div>
          <div class="meta">
            <h4 class="title">{{ m.title }}</h4>
            <p class="artist">{{ m.artist }}</p>
            <p class="likes">Likes: {{ m.like_count ?? 0 }}</p>
            <router-link :to="`/music/${m.id}`">
              <el-button size="small" type="primary">View</el-button>
            </router-link>
          </div>
        </el-card>
      </div>
    </el-card>
  </div>
  <el-button
    class="upload-fab"
    type="primary"
    circle
    :icon="Plus"
    @click="goUpload"
  />
</template>

<style scoped>
.profile-container {
  padding: 20px 0;
}
.profile-card {
  background: var(--bg-white);
}
.user-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
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
  margin: 16px 0;
}
.music-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 16px;
}
.music-card {
  display: flex;
  gap: 12px;
}
.cover {
  width: 80px;
  height: 80px;
  border-radius: 8px;
  overflow: hidden;
}
.meta .title {
  margin: 0 0 6px;
}
.artist {
  margin: 0 0 10px;
  color: var(--text-light);
}
.likes {
  margin: 0 0 6px;
  color: var(--text-light);
  font-size: 0.85rem;
}
.empty {
  margin-top: 12px;
}

.upload-fab {
  position: fixed;
  right: 24px;
  bottom: 90px;
  z-index: 1000;
}
</style>
