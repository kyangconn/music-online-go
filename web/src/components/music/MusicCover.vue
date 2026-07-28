<script setup lang="ts">
import { PictureFilled } from "@element-plus/icons-vue";
import { computed } from "vue";

const props = withDefaults(
  defineProps<{
    src?: string | null;
    alt?: string;
    preview?: boolean;
    showPlaceholderLabel?: boolean;
  }>(),
  {
    src: "",
    alt: "",
    preview: false,
    showPlaceholderLabel: false,
  },
);

const previewList = computed(() => (props.preview && props.src ? [props.src] : []));
</script>

<template>
  <el-image
    v-if="src"
    class="music-cover"
    :src="src"
    :alt="alt || $t('music.cover')"
    fit="cover"
    loading="lazy"
    :preview-src-list="previewList"
  >
    <template #error>
      <div class="cover-placeholder" role="img" :aria-label="$t('music.no_cover')">
        <el-icon><PictureFilled /></el-icon>
        <span v-if="showPlaceholderLabel">{{ $t("music.no_cover") }}</span>
      </div>
    </template>
  </el-image>
  <div v-else class="music-cover cover-placeholder" role="img" :aria-label="$t('music.no_cover')">
    <el-icon><PictureFilled /></el-icon>
    <span v-if="showPlaceholderLabel">{{ $t("music.no_cover") }}</span>
  </div>
</template>

<style scoped lang="scss">
.music-cover {
  display: block;
  width: 100%;
  height: 100%;
}

.cover-placeholder {
  @include flex-center;
  flex-direction: column;
  gap: $spacing-xs;
  width: 100%;
  height: 100%;
  min-height: 100%;
  background: $color-image-slot-bg;
  color: $color-image-slot-text;
  font-size: 28px;

  span {
    font-size: $fs-sm;
  }
}
</style>
