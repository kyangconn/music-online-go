<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import request from '@/utils/request'
import { ElMessage } from 'element-plus'
import SideNavLayout from '@/layout/SideNavLayout.vue'

const router = useRouter()
const { t } = useI18n()

const loading = ref(false)
const info = ref<any>(null)
const title = computed(() => t('admin.dashboard'))
const activeTab = ref('dashboard')
const layoutMode = ref<'sidebar' | 'tabs'>('sidebar')


const tabs = computed(() => [
  { id: 'dashboard', label: t('admin.dashboard') },
  { id: 'server', label: t('admin.server') },
  { id: 'runtime', label: t('admin.runtime') },
  { id: 'database', label: t('admin.database') },
  { id: 'music', label: t('admin.music') }
])

const handleTabChange = (tabId: string) => {
  console.log('切换到标签:', tabId)
}

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

const goBack = () => {
  router.back()
}

onMounted(fetchInfo)
</script>

<template>
  <SideNavLayout v-model="activeTab" :title="title" :tabs="tabs" :layout-mode="layoutMode" show-back-button
    @tab-change="handleTabChange" @back="goBack">
    <div v-if="loading" class="loading-container">
      <el-skeleton :rows="10" animated />
    </div>
    <div v-else-if="info" class="admin-content">
      <div v-show="activeTab === 'dashboard'">
        <div class="grid">
          <el-card class="card" shadow="hover">
            <template #header>
              <div class="card-header">
                <span>{{ $t('admin.server') }}</span>
              </div>
            </template>
            <p>Host: {{ info.host }}</p>
            <p>Mode: {{ info.server_mode }}</p>
          </el-card>

          <el-card class="card" shadow="hover">
            <template #header>
              <div class="card-header">
                <span>{{ $t('admin.runtime') }}</span>
              </div>
            </template>
            <p>Go: {{ info.go_version }}</p>
            <p>Mem: {{ (info.memory_alloc / 1024 / 1024).toFixed(1) }} MB</p>
          </el-card>

          <el-card class="card" shadow="hover">
            <template #header>
              <div class="card-header">
                <span>{{ $t('admin.database') }}</span>
              </div>
            </template>
            <p>Open: {{ info.db_open_conns }}</p>
            <p>In Use: {{ info.db_in_use }}</p>
          </el-card>

          <el-card class="card" shadow="hover">
            <template #header>
              <div class="card-header">
                <span>{{ $t('admin.music') }}</span>
              </div>
            </template>
            <p>{{ $t('admin.total') }}: {{ info.total_music_count }}</p>
          </el-card>
        </div>
      </div>

      <div v-show="activeTab === 'server'">
        <el-card class="detail-card">
          <template #header>
            <h3>{{ $t('admin.server') }}</h3>
          </template>
          <div class="detail-list">
            <div class="detail-item"><span class="label">Host:</span><span>{{ info.host }}</span></div>
            <div class="detail-item"><span class="label">Port:</span><span>{{ info.server_port }}</span></div>
            <div class="detail-item"><span class="label">Mode:</span><span>{{ info.server_mode }}</span></div>
            <div class="detail-item"><span class="label">Time:</span><span>{{ info.app_time }}</span></div>
          </div>
        </el-card>
      </div>

      <div v-show="activeTab === 'runtime'">
        <el-card class="detail-card">
          <template #header>
            <h3>{{ $t('admin.runtime') }}</h3>
          </template>
          <div class="detail-list">
            <div class="detail-item"><span class="label">Go Version:</span><span>{{ info.go_version }}</span></div>
            <div class="detail-item"><span class="label">Goroutines:</span><span>{{ info.goroutines }}</span></div>
            <div class="detail-item"><span class="label">Memory Allocated:</span><span>{{ (info.memory_alloc / 1024 /
              1024).toFixed(2) }} MB</span></div>
            <div class="detail-item"><span class="label">Memory System:</span><span>{{ (info.memory_sys / 1024 /
              1024).toFixed(2)
                }} MB</span></div>
            <div class="detail-item"><span class="label">Memory Lookups:</span><span>{{ info.memory_lookups }}</span>
            </div>
            <div class="detail-item"><span class="label">GC Counts:</span><span>{{ info.memory_gc_count }}</span></div>
          </div>
        </el-card>
      </div>

      <div v-show="activeTab === 'database'">
        <el-card class="detail-card">
          <template #header>
            <h3>{{ $t('admin.database') }}</h3>
          </template>
          <div class="detail-list">
            <div class="detail-item"><span class="label">Max Open Connections:</span><span>{{ info.db_max_open_conns
            }}</span>
            </div>
            <div class="detail-item"><span class="label">Open Connections:</span><span>{{ info.db_open_conns }}</span>
            </div>
            <div class="detail-item"><span class="label">In Use:</span><span>{{ info.db_in_use }}</span></div>
            <div class="detail-item"><span class="label">Idle:</span><span>{{ info.db_idle }}</span></div>
            <div class="detail-item"><span class="label">Wait Count:</span><span>{{ info.db_wait_count }}</span></div>
            <div class="detail-item"><span class="label">Wait Duration:</span><span>{{ info.db_wait_duration }}</span>
            </div>
          </div>
        </el-card>
      </div>

      <div v-show="activeTab === 'music'">
        <el-card class="detail-card">
          <template #header>
            <h3>{{ $t('admin.music') }}</h3>
          </template>
          <div class="detail-list">
            <div class="detail-item"><span class="label">{{ $t('admin.total') }}:</span><span>{{ info.total_music_count
            }}</span>
            </div>
          </div>
        </el-card>
      </div>
    </div>
  </SideNavLayout>
</template>

<style scoped>
.admin-content {
  padding: 16px;
}

.loading-container {
  padding: 24px;
}

.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 20px;
}

.card {
  border-radius: 12px;
  transition: transform 0.2s, box-shadow 0.2s;
}

.card:hover {
  transform: translateY(-4px);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.12);
}

.card-header {
  font-weight: 600;
  font-size: 16px;
}

.detail-card {
  max-width: 800px;
  margin: 0 auto;
  border-radius: 12px;
}

.detail-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.detail-item {
  display: flex;
  justify-content: space-between;
  padding: 8px 0;
  border-bottom: 1px solid var(--border-color);
}

.detail-item:last-child {
  border-bottom: none;
}

.label {
  font-weight: 500;
  color: var(--text-secondary);
}
</style>
