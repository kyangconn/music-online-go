<script setup lang="ts">
import { computed } from 'vue'
import { useThemeStore } from '@/store/theme'
import { Moon, Sunny, Refresh } from '@element-plus/icons-vue'

const themeStore = useThemeStore()

const themeMode = computed({
  get() {
    if (themeStore.autoSync) return 'auto'
    return themeStore.isDark ? 'dark' : 'light'
  },
  set(mode: string) {
    if (mode === 'auto') {
      themeStore.setAutoSync(true)
    } else {
      themeStore.setAutoSync(false)
      themeStore.setDarkMode(mode === 'dark')
    }
  },
})
</script>

<template>
  <div class="settings-section">
    <h3 class="section-title">{{ $t('settings.general') }}</h3>

    <div class="setting-item">
      <div class="setting-info">
        <h4>{{ $t('settings.theme_label') }}</h4>
        <p>{{ $t('settings.theme_desc') }}</p>
      </div>
      <div class="setting-control">
        <el-radio-group v-model="themeMode">
          <el-radio-button value="light">
            <el-icon><Sunny /></el-icon>
            {{ $t('settings.light') }}
          </el-radio-button>
          <el-radio-button value="dark">
            <el-icon><Moon /></el-icon>
            {{ $t('settings.dark') }}
          </el-radio-button>
          <el-radio-button value="auto">
            <el-icon><Refresh /></el-icon>
            {{ $t('settings.auto_mode') }}
          </el-radio-button>
        </el-radio-group>
      </div>
    </div>
  </div>
</template>

<style scoped>
.settings-section {
  margin-bottom: 32px;
}

.section-title {
  font-size: 18px;
  font-weight: 600;
  margin-bottom: 20px;
}

.setting-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 0;
  border-bottom: 1px solid var(--border-color);
}

.setting-control {
  display: flex;
  align-items: center;
}

.setting-info h4 {
  font-size: 16px;
  margin-bottom: 4px;
}

.setting-info p {
  font-size: 14px;
  color: var(--text-secondary);
  max-width: 360px;
}
</style>
