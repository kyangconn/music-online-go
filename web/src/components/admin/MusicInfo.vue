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
    <el-skeleton :rows="2" animated />
  </div>
  <InfoErrorState v-else-if="error || !info" @retry="$emit('retry')" />
  <div v-else class="doc-section admin-info-section">
    <h3>{{ $t("admin.music") }}</h3>
    <div class="kv-list">
      <div class="kv-row">
        <span class="kv-label">{{ $t("admin.total") }}</span
        ><span>{{ info.total_music }}</span>
      </div>
      <div class="kv-row">
        <span class="kv-label">{{ $t("admin.total_tags") }}</span
        ><span>{{ info.total_music_tags }}</span>
      </div>
    </div>
  </div>
</template>
