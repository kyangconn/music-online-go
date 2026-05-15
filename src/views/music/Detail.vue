<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useUserStore } from '@/store/user'
import request from '@/utils/request'
import { ElMessage } from 'element-plus'
import { VideoPause, VideoPlay, Star, StarFilled } from '@element-plus/icons-vue'

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()
const id = route.params.id as string
const loading = ref(true)
const music = ref<any>(null)
const audioRef = ref<HTMLAudioElement>()
const isPlaying = ref(false)
const currentTime = ref(0)
const duration = ref(0)
const volume = ref(0.8)
const isLiked = ref(false)
const likeCount = ref(0)

const audioSrc = computed(() => {
  if (!music.value) return ''
  if (music.value.path && music.value.path.startsWith('/')) {
    return music.value.path
  }
  return `/api/v1/musics/${id}/stream`
})

const fetchDetail = async () => {
  loading.value = true
  try {
    const res: any = await request.get(`/musics/${id}`)
    music.value = res.data
    isLiked.value = res.data.is_liked ?? false
    likeCount.value = res.data.like_count ?? 0
  } catch (_e) {
    ElMessage.error('Failed to load music detail')
  } finally {
    loading.value = false
  }
}

const togglePlay = () => {
  if (!audioRef.value) return
  if (audioRef.value.paused) {
    audioRef.value.play()
    isPlaying.value = true
  } else {
    audioRef.value.pause()
    isPlaying.value = false
  }
}

const onTimeUpdate = () => {
  if (audioRef.value) {
    currentTime.value = audioRef.value.currentTime
  }
}

const onLoadedMetadata = () => {
  if (audioRef.value) {
    duration.value = audioRef.value.duration
  }
}

const onEnded = () => {
  isPlaying.value = false
  currentTime.value = 0
}

const onPlay = () => {
  isPlaying.value = true
}

const onPause = () => {
  isPlaying.value = false
}

const seek = (seconds: number) => {
  if (audioRef.value) {
    audioRef.value.currentTime = seconds
  }
}

const formatTime = (seconds: number) => {
  if (!seconds || !isFinite(seconds)) return '0:00'
  const m = Math.floor(seconds / 60)
  const s = Math.floor(seconds % 60)
  return `${m}:${s.toString().padStart(2, '0')}`
}

const progressPercent = computed(() => {
  if (!duration.value) return 0
  return (currentTime.value / duration.value) * 100
})

const handleProgressClick = (e: MouseEvent) => {
  const bar = e.currentTarget as HTMLElement
  const rect = bar.getBoundingClientRect()
  const percent = (e.clientX - rect.left) / rect.width
  seek(percent * duration.value)
}

const handleLike = async () => {
  if (!userStore.isLoggedIn) {
    ElMessage.warning('Please login first')
    router.push('/login')
    return
  }
  try {
    if (isLiked.value) {
      await request.delete(`/musics/${id}/like`)
      isLiked.value = false
      likeCount.value = Math.max(0, likeCount.value - 1)
      ElMessage.success('Unliked')
    } else {
      await request.post(`/musics/${id}/like`)
      isLiked.value = true
      likeCount.value += 1
      ElMessage.success('Liked')
    }
  } catch (_e) {
    ElMessage.error('Operation failed')
  }
}

onMounted(fetchDetail)
</script>

<template>
  <div class="page-section detail-container">
    <el-card class="detail-card" shadow="never">
      <div v-if="loading" class="loading">
        <el-skeleton :rows="6" animated />
      </div>
      <div v-else class="detail-content">
        <div class="detail-top">
          <div class="cover-section">
            <div class="cover">
              <el-image
                :src="music?.img || 'https://via.placeholder.com/400x400?text=Album'"
                fit="cover"
                :preview-src-list="music?.img ? [music.img] : []"
              />
            </div>
          </div>
          <div class="info-section">
            <h1 class="music-title">{{ music?.title }}</h1>
            <p class="music-artist">{{ music?.artist }}</p>
            <el-descriptions :column="1" border>
              <el-descriptions-item label="Type">{{ music?.type }}</el-descriptions-item>
              <el-descriptions-item label="Album">{{ music?.album_id ? 'Album Track' : 'Single' }}</el-descriptions-item>
              <el-descriptions-item label="Duration">{{ formatTime(duration) }}</el-descriptions-item>
            </el-descriptions>
            <div class="likes-row">
              <el-button
                :type="isLiked ? 'warning' : 'default'"
                :icon="isLiked ? StarFilled : Star"
                @click="handleLike"
              >
                {{ likeCount }}
              </el-button>
            </div>
            <div class="actions">
              <el-button @click="router.back()">Back</el-button>
            </div>
          </div>
        </div>

        <div v-if="music?.path" class="player-section">
          <audio
            ref="audioRef"
            :src="audioSrc"
            preload="metadata"
            @timeupdate="onTimeUpdate"
            @loadedmetadata="onLoadedMetadata"
            @ended="onEnded"
            @play="onPlay"
            @pause="onPause"
          />
          <div class="player-controls">
            <el-button circle :icon="isPlaying ? VideoPause : VideoPlay" size="large" @click="togglePlay" />
            <span class="time-label current">{{ formatTime(currentTime) }}</span>
            <div class="progress-bar" @click="handleProgressClick">
              <div class="progress-track">
                <div class="progress-fill" :style="{ width: progressPercent + '%' }" />
              </div>
            </div>
            <span class="time-label total">{{ formatTime(duration) }}</span>
            <div class="volume-control">
              <span class="volume-label">🔊</span>
              <el-slider
                v-model="volume"
                :min="0"
                :max="1"
                :step="0.05"
                class="volume-slider"
                @input="(v: number) => { if (audioRef) audioRef.volume = v }"
              />
            </div>
          </div>
        </div>
        <div v-else class="no-audio-hint">
          <el-alert title="No audio file available" type="info" :closable="false" show-icon />
        </div>
      </div>
    </el-card>
  </div>
</template>

<style scoped>
.detail-card {
  background: var(--bg-white);
}

.loading {
  padding: 20px 0;
}

.detail-top {
  display: flex;
  gap: 24px;
  flex-wrap: wrap;
}

.cover-section {
  flex-shrink: 0;
}

.cover {
  width: 280px;
  border-radius: 8px;
  overflow: hidden;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}

.info-section {
  flex: 1;
  min-width: 280px;
}

.music-title {
  margin: 0 0 4px;
  font-size: 1.6rem;
  color: var(--text-dark);
}

.music-artist {
  margin: 0 0 16px;
  font-size: 1.1rem;
  color: var(--text-light);
}

.likes-row {
  margin-top: 12px;
  display: flex;
  align-items: center;
}

.actions {
  margin-top: 16px;
  display: flex;
  gap: 12px;
}

.player-section {
  margin-top: 24px;
  padding-top: 20px;
  border-top: 1px solid var(--border-color);
}

.player-controls {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 0;
}

.time-label {
  font-size: 0.85rem;
  color: var(--text-light);
  min-width: 40px;
  text-align: center;
}

.progress-bar {
  flex: 1;
  cursor: pointer;
  padding: 6px 0;
}

.progress-track {
  height: 4px;
  background: var(--border-color);
  border-radius: 2px;
  overflow: hidden;
  position: relative;
}

.progress-fill {
  height: 100%;
  background: var(--accent-color);
  border-radius: 2px;
  transition: width 0.1s linear;
}

.volume-control {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-left: auto;
}

.volume-label {
  font-size: 0.9rem;
}

.volume-slider {
  width: 100px;
}

.no-audio-hint {
  margin-top: 20px;
}

@media (max-width: 640px) {
  .detail-top {
    flex-direction: column;
  }
  .cover {
    width: 100%;
    max-width: 280px;
    margin: 0 auto;
  }
  .volume-slider {
    width: 60px;
  }
}
</style>
