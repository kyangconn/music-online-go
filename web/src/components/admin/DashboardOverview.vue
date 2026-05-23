<script setup lang="ts">
import type { SystemInfoData } from "@/types/api"

defineProps<{
  loading: boolean
  info: SystemInfoData
}>()
</script>

<template>
  <div v-if="loading" class="loading-wrap">
    <el-skeleton :rows="4" animated />
  </div>
  <div v-else-if="info" class="card-grid">
    <el-card class="card card-hover" shadow="hover">
      <template #header
        ><div class="card-header">
          <span>{{ $t("admin.server") }}</span>
        </div></template
      >
      <p>{{ $t("admin.host") }}: {{ info.host }}</p>
      <p>{{ $t("admin.mode") }}: {{ info.server_mode }}</p>
      <p>{{ $t("admin.port") }}: {{ info.server_port }}</p>
      <p>{{ $t("admin.uptime") }}: {{ info.uptime }}</p>
      <p>{{ $t("admin.app_version") }}: {{ info.app_version }}</p>
    </el-card>

    <el-card class="card card-hover" shadow="hover">
      <template #header
        ><div class="card-header">
          <span>{{ $t("admin.runtime") }}</span>
        </div></template
      >
      <p>{{ $t("admin.go_version") }}: {{ info.go_version }}</p>
      <p>{{ $t("admin.num_cpu") }}: {{ info.num_cpu }}</p>
      <p>{{ $t("admin.goroutines") }}: {{ info.goroutines }}</p>
      <p>{{ $t("admin.memory_alloc") }}: {{ info.memory_alloc }}</p>
    </el-card>

    <el-card class="card card-hover" shadow="hover">
      <template #header
        ><div class="card-header">
          <span>{{ $t("admin.database") }}</span>
        </div></template
      >
      <p>{{ $t("admin.db_type") }}: {{ info.db_type }}</p>
      <p>{{ $t("admin.db_name") }}: {{ info.db_name }}</p>
      <p>{{ $t("admin.db_open") }}: {{ info.db_open_conns }}</p>
      <p>{{ $t("admin.db_in_use") }}: {{ info.db_in_use }}</p>
      <p>{{ $t("admin.db_idle") }}: {{ info.db_idle }}</p>
      <p>{{ $t("admin.db_wait_count") }}: {{ info.db_wait_count }}</p>
    </el-card>

    <el-card class="card card-hover" shadow="hover">
      <template #header
        ><div class="card-header">
          <span>{{ $t("admin.music") }}</span>
        </div></template
      >
      <p>{{ $t("admin.total") }}: {{ info.total_music }}</p>
      <p>{{ $t("admin.total_tags") }}: {{ info.total_music_tags }}</p>
    </el-card>

    <el-card class="card card-hover" shadow="hover">
      <template #header
        ><div class="card-header">
          <span>{{ $t("admin.users") }}</span>
        </div></template
      >
      <p>{{ $t("admin.total_users") }}: {{ info.total_users }}</p>
    </el-card>
  </div>
</template>

<style scoped>
.card-header {
  font-weight: 600;
  font-size: 16px;
}

.card p {
  margin: 6px 0;
  font-size: 0.9rem;
  color: var(--text-secondary);
}
</style>
