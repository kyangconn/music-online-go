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
    <h3>Go Runtime</h3>
    <div class="kv-list">
      <div class="kv-row">
        <span class="kv-label">{{ $t("admin.go_version") }}</span
        ><span>{{ info.go_version }}</span>
      </div>
      <div class="kv-row">
        <span class="kv-label">{{ $t("admin.num_cpu") }}</span
        ><span>{{ info.num_cpu }}</span>
      </div>
      <div class="kv-row">
        <span class="kv-label">{{ $t("admin.goroutines") }}</span
        ><span>{{ info.goroutines }}</span>
      </div>
    </div>

    <h3>Memory</h3>
    <div class="kv-list">
      <div class="kv-row">
        <span class="kv-label">{{ $t("admin.memory_alloc") }}</span
        ><span>{{ info.memory_alloc }}</span>
      </div>
      <div class="kv-row">
        <span class="kv-label">{{ $t("admin.memory_total_alloc") }}</span
        ><span>{{ info.memory_total_alloc }}</span>
      </div>
      <div class="kv-row">
        <span class="kv-label">{{ $t("admin.memory_sys") }}</span
        ><span>{{ info.memory_sys }}</span>
      </div>
      <div class="kv-row">
        <span class="kv-label">{{ $t("admin.heap_alloc") }}</span
        ><span>{{ info.heap_alloc }}</span>
      </div>
      <div class="kv-row">
        <span class="kv-label">{{ $t("admin.heap_sys") }}</span
        ><span>{{ info.heap_sys }}</span>
      </div>
      <div class="kv-row">
        <span class="kv-label">{{ $t("admin.heap_idle") }}</span
        ><span>{{ info.heap_idle }}</span>
      </div>
      <div class="kv-row">
        <span class="kv-label">{{ $t("admin.heap_inuse") }}</span
        ><span>{{ info.heap_inuse }}</span>
      </div>
      <div class="kv-row">
        <span class="kv-label">{{ $t("admin.heap_released") }}</span
        ><span>{{ info.heap_released }}</span>
      </div>
      <div class="kv-row">
        <span class="kv-label">{{ $t("admin.heap_objects") }}</span
        ><span>{{ info.heap_objects }}</span>
      </div>
      <div class="kv-row">
        <span class="kv-label">{{ $t("admin.stack_inuse") }}</span
        ><span>{{ info.stack_inuse }}</span>
      </div>
      <div class="kv-row">
        <span class="kv-label">{{ $t("admin.stack_sys") }}</span
        ><span>{{ info.stack_sys }}</span>
      </div>
    </div>

    <h3>GC</h3>
    <div class="kv-list">
      <div class="kv-row">
        <span class="kv-label">{{ $t("admin.num_gc") }}</span
        ><span>{{ info.num_gc }}</span>
      </div>
      <div class="kv-row">
        <span class="kv-label">{{ $t("admin.pause_total") }}</span
        ><span>{{ info.pause_total }}</span>
      </div>
      <div class="kv-row">
        <span class="kv-label">{{ $t("admin.last_gc_time") }}</span
        ><span>{{ info.last_gc_time }}</span>
      </div>
      <div class="kv-row">
        <span class="kv-label">{{ $t("admin.gc_cpu_fraction") }}</span
        ><span>{{ info.gc_cpu_fraction }}</span>
      </div>
    </div>
  </div>
</template>
