<script setup lang="ts">
import { PictureFilled } from "@element-plus/icons-vue";
import { computed } from "vue";

const props = withDefaults(
  defineProps<{
    src?: string | null;
    preview?: boolean;
  }>(),
  {
    src: "",
    preview: false,
  },
);

const previewList = computed(() => (props.preview && props.src ? [props.src] : []));
</script>

<template>
  <el-image
    v-if="src"
    class="music-cover"
    :src="src"
    fit="cover"
    loading="lazy"
    :preview-src-list="previewList"
  >
    <template #error>
      <div class="cover-placeholder">
        <el-icon><PictureFilled /></el-icon>
      </div>
    </template>
  </el-image>
  <div v-else class="music-cover cover-placeholder">
    <el-icon><PictureFilled /></el-icon>
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
  width: 100%;
  height: 100%;
  min-height: 100%;
  background: $color-image-slot-bg;
  color: $color-image-slot-text;
  font-size: 28px;
}
</style>
