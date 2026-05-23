<script setup lang="ts">
import type { SystemInfoData } from "@/types/api"

defineProps<{
  loading: boolean
  info: SystemInfoData
}>()
</script>

<template>
  <div v-if="loading" class="loading-wrap">
    <el-skeleton :rows="2" animated />
  </div>
  <div v-else-if="info" class="doc-section">
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
