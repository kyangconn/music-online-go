<script setup lang="ts">
import { computed, watch } from 'vue'

export interface TabItem {
  id: string
  label: string
  icon?: string
  badge?: string | number
  disabled?: boolean
}

interface Props {
  // 布局模式：sidebar（侧边栏）或 tabs（标签页）
  layoutMode?: 'sidebar' | 'tabs'
  // 标题
  title?: string
  // 标签页数据
  tabs: TabItem[]
  // 当前激活的标签 (v-model)
  modelValue?: string
  // 是否显示内容标题
  showContentHeader?: boolean
  // 是否显示返回按钮
  showBackButton?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  layoutMode: 'sidebar',
  title: '设置',
  modelValue: '',
  showContentHeader: true,
  showBackButton: false,
})

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
  (e: 'tab-change', value: string): void
  (e: 'back'): void
}>()

// 当前激活标签的标签文本
const getActiveTabLabel = computed(() => {
  const active = props.tabs.find(tab => tab.id === props.modelValue)
  return active ? active.label : ''
})

// 处理标签点击
const handleTabClick = (tabId: string) => {
  if (tabId !== props.modelValue) {
    emit('update:modelValue', tabId)
    emit('tab-change', tabId)
  }
}

// 监听 tabs 变化，确保始终有激活的标签
watch(() => props.tabs, (newTabs) => {
  if (newTabs && newTabs.length > 0 && (!props.modelValue || !newTabs.find(t => t.id === props.modelValue))) {
    emit('update:modelValue', newTabs[0].id)
  }
}, { immediate: true })
</script>

<template>
  <div class="side-nav-layout" :class="layoutMode">
    <div class="layout-wrapper">
      <!-- 顶部标题栏 -->
      <header class="layout-header">
        <div class="header-left">
          <slot name="header-left">
            <button v-if="showBackButton" class="back-btn" @click="emit('back')">
              <span class="back-icon">←</span>
              <span class="back-text">{{ $t('common.back') }}</span>
            </button>
            <h1>{{ title }}</h1>
          </slot>
        </div>
        <div class="header-right">
          <slot name="header-right"></slot>
        </div>
      </header>

      <!-- 左侧导航栏 -->
      <aside class="layout-sidebar" v-if="layoutMode === 'sidebar'">
        <nav class="sidebar-nav">
          <ul>
            <li v-for="tab in tabs" :key="tab.id" :class="{
              active: modelValue === tab.id,
              disabled: tab.disabled
            }" @click="!tab.disabled && handleTabClick(tab.id)" class="nav-item">
              <span class="nav-icon">{{ tab.icon }}</span>
              <span class="nav-label">{{ tab.label }}</span>
              <span v-if="tab.badge" class="nav-badge">{{ tab.badge }}</span>
            </li>
          </ul>
        </nav>

        <div class="sidebar-footer">
          <slot name="sidebar-footer"></slot>
        </div>
      </aside>

      <!-- 标签页导航（替代侧边栏） -->
      <nav v-if="layoutMode === 'tabs'" class="layout-tabs">
        <div class="tabs-container">
          <button v-for="tab in tabs" :key="tab.id" :class="{
            'tab-button': true,
            active: modelValue === tab.id,
            disabled: tab.disabled
          }" @click="!tab.disabled && handleTabClick(tab.id)">
            <span class="tab-icon">{{ tab.icon }}</span>
            <span class="tab-label">{{ tab.label }}</span>
            <span v-if="tab.badge" class="tab-badge">{{ tab.badge }}</span>
          </button>
        </div>
      </nav>

      <!-- 主内容区 -->
      <main class="layout-main">
        <div class="main-content">
          <!-- 内容标题 -->
          <div v-if="showContentHeader" class="content-header">
            <h2>{{ getActiveTabLabel }}</h2>
            <div class="content-actions">
              <slot name="content-actions"></slot>
            </div>
          </div>

          <!-- 动态内容 -->
          <div class="content-wrapper">
            <slot :name="modelValue" :active-tab="modelValue">
              <!-- Fallback to default slot -->
              <slot></slot>
            </slot>
          </div>
        </div>
      </main>
    </div>
  </div>
</template>

<style scoped>
/* 基础布局样式 */
.side-nav-layout {
  min-height: 600px;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif;
  transition: background-color 0.3s, color 0.3s;
  background-color: var(--bg-light);
  color: var(--text-primary);
  display: flex;
  justify-content: center;
  padding: 24px 0;
}

.layout-wrapper {
  display: flex;
  flex-direction: column;
  min-height: 700px;
  width: 100%;
  max-width: 1200px;
  background-color: var(--bg-card);
  transition: background-color 0.3s;
  border: 1px solid var(--border-color);
  border-radius: 12px;
  overflow: hidden;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.05);
}

/* 头部样式 */
.layout-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 24px;
  border-bottom: 1px solid var(--border-color);
  background-color: var(--bg-card);
  z-index: 10;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 16px;
}

.header-left h1 {
  font-size: 20px;
  font-weight: 700;
  margin: 0;
}

.back-btn {
  display: flex;
  align-items: center;
  gap: 8px;
  background: none;
  border: none;
  padding: 8px 12px;
  border-radius: 8px;
  cursor: pointer;
  color: var(--text-secondary);
  transition: all 0.2s;
}

.back-btn:hover {
  background-color: var(--hover-bg);
  color: var(--text-primary);
}

.back-icon {
  font-size: 18px;
}

.back-text {
  font-size: 14px;
  font-weight: 500;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 12px;
}

/* 主内容区布局 */
.side-nav-layout.sidebar {
  .layout-wrapper {
    display: grid;
    grid-template-columns: 260px 1fr;
    grid-template-rows: auto 1fr;
    grid-template-areas:
      "header header"
      "sidebar main";
  }

  .layout-header {
    grid-area: header;
  }

  .layout-sidebar {
    grid-area: sidebar;
  }

  .layout-main {
    grid-area: main;
  }
}

.side-nav-layout.tabs {
  .layout-wrapper {
    display: grid;
    grid-template-rows: auto auto 1fr;
    grid-template-areas:
      "header"
      "tabs"
      "main";
  }

  .layout-header {
    grid-area: header;
  }

  .layout-tabs {
    grid-area: tabs;
  }

  .layout-main {
    grid-area: main;
  }
}

/* 侧边栏样式 */
.layout-sidebar {
  border-right: 1px solid var(--border-color);
  display: flex;
  flex-direction: column;
  overflow-y: auto;
  background-color: var(--bg-card);
}

.sidebar-nav ul {
  list-style: none;
  padding: 0;
  margin: 0;
}

.nav-item {
  display: flex;
  align-items: center;
  padding: 14px 20px;
  cursor: pointer;
  transition: background-color 0.2s;
  border-left: 4px solid transparent;
  position: relative;
}

.nav-item:hover:not(.disabled) {
  background-color: var(--hover-bg);
}

.nav-item.active {
  background-color: var(--active-bg);
  border-left-color: var(--accent-color);
  font-weight: 600;
}

.nav-item.disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.nav-icon {
  margin-right: 12px;
  font-size: 18px;
  width: 24px;
  text-align: center;
}

.nav-label {
  font-size: 15px;
  flex: 1;
}

.nav-badge {
  background-color: var(--accent-color);
  color: white;
  font-size: 12px;
  padding: 2px 8px;
  border-radius: 10px;
  margin-left: 8px;
}

.sidebar-footer {
  margin-top: auto;
  padding: 20px;
  border-top: 1px solid var(--border-color);
}

/* 标签页样式 */
.layout-tabs {
  border-bottom: 1px solid var(--border-color);
  background-color: var(--bg-card);
  overflow-x: auto;
}

.tabs-container {
  display: flex;
  padding: 0 20px;
}

.tab-button {
  display: flex;
  align-items: center;
  padding: 16px 24px;
  background: none;
  border: none;
  border-bottom: 3px solid transparent;
  cursor: pointer;
  transition: all 0.2s;
  color: var(--text-secondary);
  white-space: nowrap;
  font-size: 15px;
}

.tab-button:hover:not(.disabled) {
  background-color: var(--hover-bg);
  color: inherit;
}

.tab-button.active {
  color: var(--accent-color);
  border-bottom-color: var(--accent-color);
  font-weight: 600;
  background-color: var(--active-bg);
}

.tab-button.disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.tab-icon {
  margin-right: 8px;
  font-size: 16px;
}

.tab-badge {
  background-color: var(--accent-color);
  color: white;
  font-size: 12px;
  padding: 2px 8px;
  border-radius: 10px;
  margin-left: 8px;
}

/* 主内容区样式 */
.layout-main {
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.main-content {
  flex: 1;
  overflow-y: auto;
  padding: 24px;
}

.content-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
  padding-bottom: 16px;
  border-bottom: 1px solid var(--border-color);
}

.content-header h2 {
  font-size: 22px;
  font-weight: 700;
  margin: 0;
}

.content-actions {
  display: flex;
  gap: 12px;
}

.content-wrapper {
  min-height: 0;
}

/* 页脚样式 */
.layout-footer {
  padding: 16px 24px;
  border-top: 1px solid var(--border-color);
  background-color: var(--bg-card);
}

/* 响应式设计 */
@media (max-width: 768px) {
  .side-nav-layout.sidebar {
    .layout-wrapper {
      grid-template-columns: 1fr;
      grid-template-areas:
        "header"
        "main";
    }

    .layout-sidebar {
      display: none;
    }

    .layout-header {
      position: sticky;
      top: 0;
    }
  }

  .side-nav-layout.tabs {
    .tabs-container {
      padding: 0 12px;
    }

    .tab-button {
      padding: 12px 16px;
      font-size: 14px;
    }

    .tab-icon {
      margin-right: 6px;
    }
  }

  .main-content {
    padding: 16px;
  }

  .content-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 12px;
  }

  .content-actions {
    width: 100%;
    justify-content: flex-end;
  }
}
</style>
