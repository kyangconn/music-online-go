<script setup lang="ts">
import { onMounted, onUnmounted } from "vue";
import { useThemeStore } from "@/store/theme";

const themeStore = useThemeStore();

onMounted(() => {
  themeStore.init();
});

onUnmounted(() => {
  themeStore.cleanup();
});
</script>

<template>
  <router-view />
</template>

<style>
/* 全局样式已在 style.css 中定义 */
</style>
