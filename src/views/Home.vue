<script setup lang="ts">
import { ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import request from '@/utils/request'
import { ElMessage } from 'element-plus'
import { VideoPlay } from '@element-plus/icons-vue'

const route = useRoute()
const musicList = ref<any[]>([])
const loading = ref(false)
const searchQuery = ref('')
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(12)

const fetchMusic = async () => {
  loading.value = true
  try {
    const params = {
      q: searchQuery.value,
      page: currentPage.value,
      page_size: pageSize.value
    }
    const res: any = await request.get('/musics', { params })
    musicList.value = res.data.items || []
    total.value = res.data.total
  } catch (error) {
    console.error(error)
    ElMessage.error('Failed to load music list')
  } finally {
    loading.value = false
  }
}

watch(() => route.query.q, (newQ) => {
  searchQuery.value = newQ as string || ''
  currentPage.value = 1
  fetchMusic()
}, { immediate: true })

const handlePageChange = (page: number) => {
  currentPage.value = page
  fetchMusic()
}
</script>

<template>
  <div class="home-container">
    <div class="banner">
      <h1>Discover New Music</h1>
      <p>Listen to the latest tracks from our community</p>
    </div>

    <div class="music-section">
      <h2 v-if="searchQuery">Search Results for "{{ searchQuery }}"</h2>
      <h2 v-else>Recommended</h2>
      
      <div v-if="loading" class="loading-container">
        <el-skeleton :rows="3" animated />
      </div>
      
      <div v-else-if="musicList.length === 0" class="empty-state">
        <el-empty description="No music found" />
      </div>

      <div v-else class="music-grid">
        <el-card 
          v-for="music in musicList" 
          :key="music.id" 
          class="music-card" 
          :body-style="{ padding: '0px' }"
          shadow="hover"
        >
          <div class="cover-image">
            <el-image 
              :src="music.cover_url || 'https://via.placeholder.com/300x300?text=Music'" 
              fit="cover"
              loading="lazy"
            >
              <template #error>
                <div class="image-slot">
                  <el-icon><icon-picture /></el-icon>
                </div>
              </template>
            </el-image>
            <div class="play-overlay">
              <router-link :to="`/music/${music.id}`">
                <el-button type="primary" circle :icon="VideoPlay" size="large" />
              </router-link>
            </div>
          </div>
          <div class="music-info">
            <h3 class="music-title" :title="music.title">{{ music.title }}</h3>
            <p class="music-artist">{{ music.artist }}</p>
          </div>
        </el-card>
      </div>

      <div class="pagination-container" v-if="total > pageSize">
        <el-pagination
          background
          layout="prev, pager, next"
          :total="total"
          :page-size="pageSize"
          v-model:current-page="currentPage"
          @current-change="handlePageChange"
        />
      </div>
    </div>
  </div>
</template>

<style scoped>
.banner {
  background: linear-gradient(135deg, var(--secondary-color), var(--primary-color));
  color: white;
  padding: 3rem 2rem;
  border-radius: 8px;
  margin-bottom: 2rem;
  text-align: center;
}

.banner h1 {
  margin: 0 0 1rem;
  font-size: 2.5rem;
}

.banner p {
  margin: 0;
  opacity: 0.8;
  font-size: 1.2rem;
}

.music-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 20px;
  margin-bottom: 2rem;
}

.music-card {
  transition: transform 0.3s;
  border: none;
  background: var(--bg-white);
}

.music-card:hover {
  transform: translateY(-5px);
}

.cover-image {
  position: relative;
  width: 100%;
  padding-top: 100%; /* 1:1 Aspect Ratio */
  background-color: #f0f0f0;
  overflow: hidden;
}

.cover-image .el-image {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
}

.play-overlay {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background: rgba(0, 0, 0, 0.3);
  display: flex;
  justify-content: center;
  align-items: center;
  opacity: 0;
  transition: opacity 0.3s;
}

.music-card:hover .play-overlay {
  opacity: 1;
}

.music-info {
  padding: 12px;
}

.music-title {
  margin: 0 0 5px;
  font-size: 1rem;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  color: var(--text-dark);
}

.music-artist {
  margin: 0;
  font-size: 0.85rem;
  color: var(--text-light);
}

.pagination-container {
  display: flex;
  justify-content: center;
  margin-top: 2rem;
}

.image-slot {
  display: flex;
  justify-content: center;
  align-items: center;
  width: 100%;
  height: 100%;
  background: #f5f7fa;
  color: #909399;
}
</style>
