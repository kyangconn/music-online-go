<script setup lang="ts">
import { RefreshRight } from "@element-plus/icons-vue";

withDefaults(
  defineProps<{
    description?: string;
  }>(),
  {
    description: "",
  },
);

defineEmits<{
  retry: [];
}>();
</script>

<template>
  <div class="info-error-state">
    <el-result icon="warning" :title="$t('common.load_failed')" :sub-title="description || $t('common.load_failed_desc')">
      <template #extra>
        <el-button type="primary" :icon="RefreshRight" @click="$emit('retry')">
          {{ $t("common.retry") }}
        </el-button>
      </template>
    </el-result>
  </div>
</template>

<style scoped lang="scss">
.info-error-state {
  width: 100%;
  padding: $spacing-lg 0;

  :deep(.el-result) {
    padding: $spacing-xl $spacing-md;
  }
}
</style>
