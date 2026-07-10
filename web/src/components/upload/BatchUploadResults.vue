<script setup lang="ts">
import { Delete, RefreshRight, View } from "@element-plus/icons-vue";
import { computed } from "vue";
import { useRouter } from "vue-router";
import type { BatchUploadResultItem, BatchUploadStatus } from "@/types/upload";

const props = defineProps<{
  results: BatchUploadResultItem[];
  uploading: boolean;
}>();

const emit = defineEmits<{
  retry: [path: string];
  retryAll: [];
  clear: [];
}>();

const router = useRouter();
const failedCount = computed(() => props.results.filter((item) => item.status === "failed").length);
const successCount = computed(() => props.results.filter((item) => item.status === "success").length);
const skippedCount = computed(() => props.results.filter((item) => item.status === "skipped").length);

const tagType = (status: BatchUploadStatus) => {
  if (status === "success") return "success";
  if (status === "failed") return "danger";
  if (status === "skipped") return "warning";
  return "info";
};

const openMusic = (musicId?: number) => {
  if (musicId) void router.push(`/music/${musicId}`);
};
</script>

<template>
  <section v-if="results.length" class="upload-results" aria-live="polite">
    <div class="results-heading">
      <div>
        <h3>{{ $t("add.batch_results") }}</h3>
        <div class="result-summary">
          <el-tag type="success" size="small">{{ $t("add.result_success_count", { count: successCount }) }}</el-tag>
          <el-tag type="warning" size="small">{{ $t("add.result_skipped_count", { count: skippedCount }) }}</el-tag>
          <el-tag v-if="failedCount" type="danger" size="small">
            {{ $t("add.result_failed_count", { count: failedCount }) }}
          </el-tag>
        </div>
      </div>
      <div class="result-actions">
        <el-tooltip v-if="failedCount" :content="$t('add.retry_all_failed')" placement="top">
          <el-button
            circle
            type="primary"
            plain
            :icon="RefreshRight"
            :loading="uploading"
            :aria-label="$t('add.retry_all_failed')"
            @click="emit('retryAll')"
          />
        </el-tooltip>
        <el-tooltip :content="$t('add.clear_results')" placement="top">
          <el-button
            circle
            plain
            :icon="Delete"
            :disabled="uploading"
            :aria-label="$t('add.clear_results')"
            @click="emit('clear')"
          />
        </el-tooltip>
      </div>
    </div>

    <el-table :data="results" size="small">
      <el-table-column prop="name" :label="$t('add.file_name')" min-width="180" show-overflow-tooltip />
      <el-table-column :label="$t('add.result_status')" width="110">
        <template #default="{ row }">
          <el-tag :type="tagType(row.status)" size="small">
            {{ $t(`add.result_${row.status}`) }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="reason" :label="$t('add.result_reason')" min-width="220" show-overflow-tooltip />
      <el-table-column :label="$t('add.result_attempts')" width="80" align="center">
        <template #default="{ row }">{{ row.attempts }}</template>
      </el-table-column>
      <el-table-column width="96" align="right">
        <template #default="{ row }">
          <el-tooltip v-if="row.status === 'failed'" :content="$t('add.retry_failed')" placement="top">
            <el-button
              circle
              text
              :icon="RefreshRight"
              :disabled="uploading"
              :aria-label="$t('add.retry_failed')"
              @click="emit('retry', row.path)"
            />
          </el-tooltip>
          <el-tooltip v-if="row.musicId" :content="$t('common.view')" placement="top">
            <el-button
              circle
              text
              :icon="View"
              :aria-label="$t('common.view')"
              @click="openMusic(row.musicId)"
            />
          </el-tooltip>
        </template>
      </el-table-column>
    </el-table>
  </section>
</template>

<style scoped lang="scss">
.upload-results {
  margin-bottom: $spacing-xl;
  padding: $spacing-lg 0;
  border-top: 1px solid var(--border-color);
  border-bottom: 1px solid var(--border-color);
}

.results-heading {
  @include flex-between;
  gap: $spacing-md;
  margin-bottom: $spacing-md;

  h3 {
    margin: 0 0 $spacing-sm;
  }
}

.result-summary,
.result-actions {
  display: flex;
  align-items: center;
  gap: $spacing-sm;
}
</style>
