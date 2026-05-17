<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import request from '@/utils/request'
import { ElMessage } from 'element-plus'
import SideNavLayout, { type TabItem } from '@/layout/SideNavLayout.vue'

const router = useRouter()
const { t } = useI18n()

const loading = ref(false)
const info = ref<any>(null)
const title = computed(() => t('admin.dashboard'))
const activeTab = ref('dashboard')
const layoutMode = ref<'sidebar' | 'tabs'>('sidebar')

const tabs = computed<TabItem[]>(() => [
  { id: 'dashboard', label: t('admin.dashboard') },
  { id: 'server', label: t('admin.server') },
  { id: 'runtime', label: t('admin.runtime') },
  { id: 'database', label: t('admin.database') },
  { id: 'music', label: t('admin.music') },
])

const fetchInfo = async () => {
  loading.value = true
  try {
    const res: any = await request.get('/users/admin/system-info')
    info.value = res.data
  } catch (_e) {
    ElMessage.error('Failed to load system info')
  } finally {
    loading.value = false
  }
}

const goBack = () => router.back()

onMounted(fetchInfo)
</script>

<template>
  <SideNavLayout
    v-model="activeTab"
    :title="title"
    :tabs="tabs"
    :layout-mode="layoutMode"
    :show-content-header="false"
    show-back-button
    @back="goBack"
  >
    <template #dashboard>
      <div v-if="loading" class="loading-wrap"><el-skeleton :rows="4" animated /></div>
      <div v-else-if="info" class="card-grid">
        <el-card class="card card-hover" shadow="hover">
          <template #header><div class="card-header"><span>{{ $t('admin.server') }}</span></div></template>
          <p>{{ $t('admin.host') }}: {{ info.host }}</p>
          <p>{{ $t('admin.mode') }}: {{ info.server_mode }}</p>
          <p>{{ $t('admin.port') }}: {{ info.server_port }}</p>
          <p>{{ $t('admin.uptime') }}: {{ info.uptime }}</p>
          <p>{{ $t('admin.app_version') }}: {{ info.app_version }}</p>
        </el-card>

        <el-card class="card card-hover" shadow="hover">
          <template #header><div class="card-header"><span>{{ $t('admin.runtime') }}</span></div></template>
          <p>{{ $t('admin.go_version') }}: {{ info.go_version }}</p>
          <p>{{ $t('admin.num_cpu') }}: {{ info.num_cpu }}</p>
          <p>{{ $t('admin.goroutines') }}: {{ info.goroutines }}</p>
          <p>{{ $t('admin.memory_alloc') }}: {{ info.memory_alloc }}</p>
        </el-card>

        <el-card class="card card-hover" shadow="hover">
          <template #header><div class="card-header"><span>{{ $t('admin.database') }}</span></div></template>
          <p>{{ $t('admin.db_type') }}: {{ info.db_type }}</p>
          <p>{{ $t('admin.db_name') }}: {{ info.db_name }}</p>
          <p>{{ $t('admin.db_open') }}: {{ info.db_open_conns }}</p>
          <p>{{ $t('admin.db_in_use') }}: {{ info.db_in_use }}</p>
          <p>{{ $t('admin.db_idle') }}: {{ info.db_idle }}</p>
          <p>{{ $t('admin.db_wait_count') }}: {{ info.db_wait_count }}</p>
        </el-card>

        <el-card class="card card-hover" shadow="hover">
          <template #header><div class="card-header"><span>{{ $t('admin.music') }}</span></div></template>
          <p>{{ $t('admin.total') }}: {{ info.total_music }}</p>
          <p>{{ $t('admin.total_tags') }}: {{ info.total_music_tags }}</p>
        </el-card>

        <el-card class="card card-hover" shadow="hover">
          <template #header><div class="card-header"><span>{{ $t('admin.users') }}</span></div></template>
          <p>{{ $t('admin.total_users') }}: {{ info.total_users }}</p>
        </el-card>
      </div>
    </template>

    <template #server>
      <div v-if="loading" class="loading-wrap"><el-skeleton :rows="3" animated /></div>
      <div v-else-if="info" class="doc-section">
        <h3>{{ $t('admin.server') }}</h3>
        <div class="kv-list">
          <div class="kv-row"><span class="kv-label">{{ $t('admin.host') }}</span><span>{{ info.host }}</span></div>
          <div class="kv-row"><span class="kv-label">{{ $t('admin.port') }}</span><span>{{ info.server_port }}</span></div>
          <div class="kv-row"><span class="kv-label">{{ $t('admin.mode') }}</span><span>{{ info.server_mode }}</span></div>
          <div class="kv-row"><span class="kv-label">App Time</span><span>{{ info.app_time }}</span></div>
          <div class="kv-row"><span class="kv-label">{{ $t('admin.uptime') }}</span><span>{{ info.uptime }}</span></div>
          <div class="kv-row"><span class="kv-label">{{ $t('admin.app_version') }}</span><span>{{ info.app_version }}</span></div>
          <div class="kv-row"><span class="kv-label">{{ $t('admin.app_commit') }}</span><span>{{ info.app_commit }}</span></div>
          <div class="kv-row"><span class="kv-label">{{ $t('admin.app_built') }}</span><span>{{ info.app_built }}</span></div>
        </div>
      </div>
    </template>

    <template #runtime>
      <div v-if="loading" class="loading-wrap"><el-skeleton :rows="3" animated /></div>
      <div v-else-if="info" class="doc-section">
        <h3>Go Runtime</h3>
        <div class="kv-list">
          <div class="kv-row"><span class="kv-label">{{ $t('admin.go_version') }}</span><span>{{ info.go_version }}</span></div>
          <div class="kv-row"><span class="kv-label">{{ $t('admin.num_cpu') }}</span><span>{{ info.num_cpu }}</span></div>
          <div class="kv-row"><span class="kv-label">{{ $t('admin.goroutines') }}</span><span>{{ info.goroutines }}</span></div>
        </div>

        <h3>Memory</h3>
        <div class="kv-list">
          <div class="kv-row"><span class="kv-label">{{ $t('admin.memory_alloc') }}</span><span>{{ info.memory_alloc }}</span></div>
          <div class="kv-row"><span class="kv-label">{{ $t('admin.memory_total_alloc') }}</span><span>{{ info.memory_total_alloc }}</span></div>
          <div class="kv-row"><span class="kv-label">{{ $t('admin.memory_sys') }}</span><span>{{ info.memory_sys }}</span></div>
          <div class="kv-row"><span class="kv-label">{{ $t('admin.heap_alloc') }}</span><span>{{ info.heap_alloc }}</span></div>
          <div class="kv-row"><span class="kv-label">{{ $t('admin.heap_sys') }}</span><span>{{ info.heap_sys }}</span></div>
          <div class="kv-row"><span class="kv-label">{{ $t('admin.heap_idle') }}</span><span>{{ info.heap_idle }}</span></div>
          <div class="kv-row"><span class="kv-label">{{ $t('admin.heap_inuse') }}</span><span>{{ info.heap_inuse }}</span></div>
          <div class="kv-row"><span class="kv-label">{{ $t('admin.heap_released') }}</span><span>{{ info.heap_released }}</span></div>
          <div class="kv-row"><span class="kv-label">{{ $t('admin.heap_objects') }}</span><span>{{ info.heap_objects }}</span></div>
          <div class="kv-row"><span class="kv-label">{{ $t('admin.stack_inuse') }}</span><span>{{ info.stack_inuse }}</span></div>
          <div class="kv-row"><span class="kv-label">{{ $t('admin.stack_sys') }}</span><span>{{ info.stack_sys }}</span></div>
        </div>

        <h3>GC</h3>
        <div class="kv-list">
          <div class="kv-row"><span class="kv-label">{{ $t('admin.num_gc') }}</span><span>{{ info.num_gc }}</span></div>
          <div class="kv-row"><span class="kv-label">{{ $t('admin.pause_total') }}</span><span>{{ info.pause_total }}</span></div>
          <div class="kv-row"><span class="kv-label">{{ $t('admin.last_gc_time') }}</span><span>{{ info.last_gc_time }}</span></div>
          <div class="kv-row"><span class="kv-label">{{ $t('admin.gc_cpu_fraction') }}</span><span>{{ info.gc_cpu_fraction }}</span></div>
        </div>
      </div>
    </template>

    <template #database>
      <div v-if="loading" class="loading-wrap"><el-skeleton :rows="3" animated /></div>
      <div v-else-if="info" class="doc-section">
        <h3>{{ $t('admin.database') }}</h3>
        <div class="kv-list">
          <div class="kv-row"><span class="kv-label">{{ $t('admin.db_type') }}</span><span>{{ info.db_type }}</span></div>
          <div class="kv-row"><span class="kv-label">{{ $t('admin.db_name') }}</span><span>{{ info.db_name }}</span></div>
          <div class="kv-row"><span class="kv-label">{{ $t('admin.db_max_open') }}</span><span>{{ info.db_max_open_conns }}</span></div>
          <div class="kv-row"><span class="kv-label">{{ $t('admin.db_open') }}</span><span>{{ info.db_open_conns }}</span></div>
          <div class="kv-row"><span class="kv-label">{{ $t('admin.db_in_use') }}</span><span>{{ info.db_in_use }}</span></div>
          <div class="kv-row"><span class="kv-label">{{ $t('admin.db_idle') }}</span><span>{{ info.db_idle }}</span></div>
          <div class="kv-row"><span class="kv-label">{{ $t('admin.db_wait_count') }}</span><span>{{ info.db_wait_count }}</span></div>
          <div class="kv-row"><span class="kv-label">{{ $t('admin.db_wait_duration') }}</span><span>{{ info.db_wait_duration }}</span></div>
        </div>
      </div>
    </template>

    <template #music>
      <div v-if="loading" class="loading-wrap"><el-skeleton :rows="2" animated /></div>
      <div v-else-if="info" class="doc-section">
        <h3>{{ $t('admin.music') }}</h3>
        <div class="kv-list">
          <div class="kv-row"><span class="kv-label">{{ $t('admin.total') }}</span><span>{{ info.total_music }}</span></div>
          <div class="kv-row"><span class="kv-label">{{ $t('admin.total_tags') }}</span><span>{{ info.total_music_tags }}</span></div>
        </div>
      </div>
    </template>
  </SideNavLayout>
</template>

<style scoped>
.loading-wrap {
  padding: 16px 0;
}

.card-header {
  font-weight: 600;
  font-size: 16px;
}

.card p {
  margin: 6px 0;
  font-size: 0.9rem;
  color: var(--text-secondary);
}

.doc-section h3 {
  font-size: 1.1rem;
  font-weight: 600;
  margin: 0 0 12px;
  color: var(--text-primary);
}

.doc-section h3:not(:first-child) {
  margin-top: 28px;
}

.doc-section .kv-list {
  max-width: 600px;
}
</style>
