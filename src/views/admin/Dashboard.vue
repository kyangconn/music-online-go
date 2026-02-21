<script setup lang="ts">
import { ref, onMounted } from 'vue'
import request from '@/utils/request'
import { ElMessage } from 'element-plus'

const loading = ref(false)
const info = ref<any>(null)

const fetchInfo = async () => {
  loading.value = true
  try {
    const res: any = await request.get('/users/admin/system-info')
    info.value = res.data
  } catch (e) {
    ElMessage.error('Failed to load system info')
  } finally {
    loading.value = false
  }
}

onMounted(fetchInfo)
</script>

<template>
  <div class="admin-container">
    <el-page-header content="Admin Dashboard" />

    <el-skeleton v-if="loading" :rows="4" animated />
    <div v-else-if="info" class="grid">
      <el-card class="card" shadow="hover">
        <h3>Server</h3>
        <p>Host: {{ info.host }}</p>
        <p>Mode: {{ info.server_mode }}</p>
        <p>Port: {{ info.server_port }}</p>
        <p>Time: {{ info.app_time }}</p>
      </el-card>

      <el-card class="card" shadow="hover">
        <h3>Runtime</h3>
        <p>Go: {{ info.go_version }}</p>
        <p>Goroutines: {{ info.goroutines }}</p>
        <p>Mem Alloc: {{ (info.memory_alloc / 1024 / 1024).toFixed(1) }} MB</p>
        <p>Mem Sys: {{ (info.memory_sys / 1024 / 1024).toFixed(1) }} MB</p>
      </el-card>

      <el-card class="card" shadow="hover">
        <h3>Database</h3>
        <p>Max Open: {{ info.db_max_open_conns }}</p>
        <p>Open: {{ info.db_open_conns }}</p>
        <p>In Use: {{ info.db_in_use }}</p>
        <p>Idle: {{ info.db_idle }}</p>
      </el-card>

      <el-card class="card" shadow="hover">
        <h3>Music</h3>
        <p>Total: {{ info.total_music_count }}</p>
      </el-card>
    </div>
  </div>
</template>

<style scoped>
.admin-container {
  padding: 20px 0;
}
.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 16px;
  margin-top: 16px;
}
.card h3 {
  margin: 0 0 8px;
}
.card p {
  margin: 0 0 4px;
}
</style>
