<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import request from '@/utils/request'
import { ElMessage } from 'element-plus'

const route = useRoute()
const id = route.params.id as string
const loading = ref(true)
const music = ref<any>(null)

const fetchDetail = async () => {
  loading.value = true
  try {
    const res: any = await request.get(`/musics/${id}`)
    music.value = res.data
  } catch (e) {
    ElMessage.error('Failed to load music detail')
  } finally {
    loading.value = false
  }
}

onMounted(fetchDetail)
</script>

<template>
  <div class="detail-container">
    <el-card class="detail-card" shadow="never">
      <div v-if="loading" class="loading">
        <el-skeleton :rows="6" animated />
      </div>
      <div v-else>
        <el-row :gutter="20">
          <el-col :xs="24" :sm="10" :md="8" :lg="6">
            <div class="cover">
              <el-image
                :src="music?.cover_url || 'https://via.placeholder.com/400x400?text=Album'"
                fit="cover"
                :preview-src-list="[music?.cover_url]"
              />
            </div>
          </el-col>
          <el-col :xs="24" :sm="14" :md="16" :lg="18">
            <el-descriptions title="Music Info" :column="1" border>
              <el-descriptions-item label="Title">{{ music?.title }}</el-descriptions-item>
              <el-descriptions-item label="Artist">{{ music?.artist }}</el-descriptions-item>
              <el-descriptions-item label="Album">{{ music?.album || '—' }}</el-descriptions-item>
              <el-descriptions-item label="Duration">{{ music?.duration || '—' }}</el-descriptions-item>
              <el-descriptions-item label="Uploader">{{ music?.uploader?.username || '—' }}</el-descriptions-item>
            </el-descriptions>
            <div class="actions">
              <el-button type="primary">Play</el-button>
              <el-button @click="$router.back()">Back</el-button>
            </div>
          </el-col>
        </el-row>
      </div>
    </el-card>
  </div>
</template>

<style scoped>
.detail-container {
  padding: 20px 0;
}
.detail-card {
  background: var(--bg-white);
}
.cover {
  border-radius: 8px;
  overflow: hidden;
  box-shadow: 0 4px 12px rgba(0,0,0,0.1);
}
.actions {
  margin-top: 16px;
  display: flex;
  gap: 12px;
}
</style>
