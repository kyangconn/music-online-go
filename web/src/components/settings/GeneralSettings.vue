<script setup lang="ts">
import { Moon, Sunny, Refresh } from "@element-plus/icons-vue"
import { computed } from "vue"
import { useThemeStore } from "@/store/theme"

const themeStore = useThemeStore()

const themeMode = computed({
  get() {
    if (themeStore.autoSync) return "auto"
    return themeStore.isDark ? "dark" : "light"
  },
  set(mode: string) {
    if (mode === "auto") {
      themeStore.setAutoSync(true)
    } else {
      themeStore.setAutoSync(false)
      themeStore.setDarkMode(mode === "dark")
    }
  },
})
</script>

<template>
  <div class="settings-section">
    <h3 class="section-title">{{ $t("settings.general") }}</h3>

    <div class="setting-item">
      <div class="setting-info">
        <h4>{{ $t("settings.theme_label") }}</h4>
        <p>{{ $t("settings.theme_desc") }}</p>
      </div>
      <div class="setting-control">
        <el-radio-group v-model="themeMode">
          <el-radio-button value="light">
            <el-icon><Sunny /></el-icon>
            {{ $t("settings.light") }}
          </el-radio-button>
          <el-radio-button value="dark">
            <el-icon><Moon /></el-icon>
            {{ $t("settings.dark") }}
          </el-radio-button>
          <el-radio-button value="auto">
            <el-icon><Refresh /></el-icon>
            {{ $t("settings.auto_mode") }}
          </el-radio-button>
        </el-radio-group>
      </div>
    </div>
  </div>
</template>

<style scoped lang="scss">
.setting-control {
  display: flex;
  align-items: center;
}

.setting-info p {
  max-width: 360px;
}
</style>
