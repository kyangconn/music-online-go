<script setup lang="ts">
import type { SystemInfoData } from "@/types/api";

defineProps<{
  loading: boolean;
  info: SystemInfoData;
}>();
</script>

<template>
  <div v-if="loading" class="loading-wrap">
    <el-skeleton :rows="3" animated />
  </div>
  <div v-else-if="info" class="doc-section">
    <h3>{{ $t("admin.database") }}</h3>
    <div class="kv-list">
      <div class="kv-row">
        <span class="kv-label">{{ $t("admin.db_type") }}</span
        ><span>{{ info.db_type }}</span>
      </div>
      <div class="kv-row">
        <span class="kv-label">{{ $t("admin.db_name") }}</span
        ><span>{{ info.db_name }}</span>
      </div>
      <div class="kv-row">
        <span class="kv-label">{{ $t("admin.db_max_open") }}</span
        ><span>{{ info.db_max_open_conns }}</span>
      </div>
      <div class="kv-row">
        <span class="kv-label">{{ $t("admin.db_open") }}</span
        ><span>{{ info.db_open_conns }}</span>
      </div>
      <div class="kv-row">
        <span class="kv-label">{{ $t("admin.db_in_use") }}</span
        ><span>{{ info.db_in_use }}</span>
      </div>
      <div class="kv-row">
        <span class="kv-label">{{ $t("admin.db_idle") }}</span
        ><span>{{ info.db_idle }}</span>
      </div>
      <div class="kv-row">
        <span class="kv-label">{{ $t("admin.db_wait_count") }}</span
        ><span>{{ info.db_wait_count }}</span>
      </div>
      <div class="kv-row">
        <span class="kv-label">{{ $t("admin.db_wait_duration") }}</span
        ><span>{{ info.db_wait_duration }}</span>
      </div>
    </div>
  </div>
</template>
