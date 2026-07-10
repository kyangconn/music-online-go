<script setup lang="ts">
import type { SystemInfoData } from "@/types/api";
import InfoErrorState from "@/components/common/InfoErrorState.vue";

defineProps<{
  loading: boolean;
  info: SystemInfoData | null;
  error: boolean;
}>();

defineEmits<{
  retry: [];
}>();
</script>

<template>
  <div v-if="loading" class="loading-wrap">
    <el-skeleton :rows="3" animated />
  </div>
  <InfoErrorState v-else-if="error || !info" @retry="$emit('retry')" />
  <div v-else class="doc-section admin-info-section">
    <h3>{{ $t("admin.server") }}</h3>
    <div class="kv-list">
      <div class="kv-row">
        <span class="kv-label">{{ $t("admin.host") }}</span
        ><span>{{ info.host }}</span>
      </div>
      <div class="kv-row">
        <span class="kv-label">{{ $t("admin.port") }}</span
        ><span>{{ info.server_port }}</span>
      </div>
      <div class="kv-row">
        <span class="kv-label">{{ $t("admin.mode") }}</span
        ><span>{{ info.server_mode }}</span>
      </div>
      <div class="kv-row">
        <span class="kv-label">App Time</span><span>{{ info.app_time }}</span>
      </div>
      <div class="kv-row">
        <span class="kv-label">{{ $t("admin.uptime") }}</span
        ><span>{{ info.uptime }}</span>
      </div>
      <div class="kv-row">
        <span class="kv-label">{{ $t("admin.app_version") }}</span
        ><span>{{ info.app_version }}</span>
      </div>
      <div class="kv-row">
        <span class="kv-label">{{ $t("admin.app_commit") }}</span
        ><span>{{ info.app_commit }}</span>
      </div>
      <div class="kv-row">
        <span class="kv-label">{{ $t("admin.app_built") }}</span
        ><span>{{ info.app_built }}</span>
      </div>
    </div>
  </div>
</template>
