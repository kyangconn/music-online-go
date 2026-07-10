<script setup lang="ts">
import { Close, UploadFilled } from "@element-plus/icons-vue";
import { formatFileSize } from "@/utils/upload";

defineProps<{
  label: string;
  icon?: unknown;
  file: File | null;
  /** 未选择文件时展示的占位文案 */
  placeholder?: string;
}>();

defineEmits<{
  select: [];
  remove: [];
}>();
</script>

<template>
  <div
    class="file-card"
    :class="{ filled: file }"
    role="button"
    tabindex="0"
    @click="$emit('select')"
    @keydown.enter.prevent="$emit('select')"
  >
    <div class="file-card-body">
      <el-icon :size="28"><component :is="icon" /></el-icon>
      <div class="file-card-info">
        <p class="file-card-label">{{ label }}</p>
        <p class="file-card-name">{{ file ? file.name : (placeholder ?? $t("common.not_selected")) }}</p>
        <p v-if="file" class="file-card-size">{{ formatFileSize(file.size) }}</p>
      </div>
    </div>
    <button v-if="file" class="dismiss-btn" :aria-label="$t('common.delete')" @click.stop="$emit('remove')">
      <el-icon :size="14"><Close /></el-icon>
    </button>
    <el-icon v-if="!file" class="card-upload-icon" :size="20"><UploadFilled /></el-icon>
  </div>
</template>

<style scoped lang="scss">
.file-card {
  flex: 1;
  position: relative;
  min-height: 88px;
  border: 2px dashed var(--border-color);
  border-radius: $radius-xl;
  padding: $spacing-lg;
  @include inline-flex;
  transition:
    border-color $transition-base,
    background $transition-base;
  background: var(--bg-white);

  &.filled {
    border-style: solid;
    border-color: var(--accent-color);
    background: color-mix(in srgb, var(--accent-color) 4%, var(--bg-white));
  }
  &:hover {
    border-color: var(--accent-color);
  }
}

.file-card-body {
  @include inline-flex($spacing-md);
  flex: 1;
  color: var(--text-secondary);
  min-width: 0;
  padding-right: $spacing-3xl;
}

.file-card.filled .file-card-body {
  color: var(--text-primary);
}

.file-card-info {
  min-width: 0;
}

.file-card-label {
  font-size: 0.8rem;
  font-weight: $fw-semibold;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: var(--text-light);
  margin: 0 0 $spacing-xs;
}

.file-card-name {
  margin: 0;
  font-size: $fs-base;
  @include text-ellipsis;
}

.file-card-size {
  margin: $spacing-xs 0 0;
  font-size: $fs-xs;
  color: var(--text-light);
}

.dismiss-btn {
  position: absolute;
  top: $spacing-sm;
  right: $spacing-sm;
  width: $spacing-2xl;
  height: $spacing-2xl;
  border-radius: $radius-round;
  border: 2px solid var(--bg-white);
  background: $color-danger;
  color: #fff;
  cursor: pointer;
  @include flex-center;
  padding: 0;
  transition:
    transform 0.15s,
    box-shadow 0.15s;
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.15);
  z-index: 3;

  &:hover {
    transform: scale(1.15);
    box-shadow: 0 4px 12px rgba($color-danger, 0.4);
  }
}

.card-upload-icon {
  position: absolute;
  top: 50%;
  right: $spacing-lg;
  transform: translateY(-50%);
  color: var(--text-light);
  transition: color $transition-base;
  pointer-events: none;
  z-index: 2;
}

.file-card:hover .card-upload-icon {
  color: var(--accent-color);
}
</style>
