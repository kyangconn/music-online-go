<script setup lang="ts">
import { ArrowLeft } from "@element-plus/icons-vue"
import { computed, watch } from "vue"

export interface TabItem {
  id: string
  label: string
  icon?: string
  badge?: string | number
  disabled?: boolean
}

interface Props {
  layoutMode?: "sidebar" | "tabs"
  title?: string
  tabs: TabItem[]
  modelValue?: string
  showContentHeader?: boolean
  showBackButton?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  layoutMode: "sidebar",
  title: "设置",
  modelValue: "",
  showContentHeader: true,
  showBackButton: false,
})

const emit = defineEmits<{
  (e: "update:modelValue", value: string): void
  (e: "tab-change", value: string): void
  (e: "back"): void
}>()

const getActiveTabLabel = computed(() => {
  const active = props.tabs.find((tab) => tab.id === props.modelValue)
  return active ? active.label : ""
})

const handleTabClick = (tabId: string) => {
  if (tabId !== props.modelValue) {
    emit("update:modelValue", tabId)
    emit("tab-change", tabId)
  }
}

watch(
  () => props.tabs,
  (newTabs) => {
    if (newTabs && newTabs.length > 0 && (!props.modelValue || !newTabs.find((t) => t.id === props.modelValue))) {
      emit("update:modelValue", newTabs[0]!.id)
    }
  },
  { immediate: true },
)
</script>

<template>
  <div class="side-nav-layout" :class="layoutMode">
    <div class="layout-wrapper">
      <header class="layout-header">
        <div class="header-left">
          <slot name="header-left">
            <button v-if="showBackButton" class="back-btn" @click="emit('back')">
              <el-icon class="back-icon"><ArrowLeft /></el-icon>
              <span class="back-text">{{ $t("common.back") }}</span>
            </button>
            <h1>{{ title }}</h1>
          </slot>
        </div>
        <div class="header-right">
          <slot name="header-right"></slot>
        </div>
      </header>

      <aside class="layout-sidebar" v-if="layoutMode === 'sidebar'">
        <nav class="sidebar-nav">
          <ul>
            <li
              v-for="tab in tabs"
              :key="tab.id"
              :class="{
                active: modelValue === tab.id,
                disabled: tab.disabled,
              }"
              @click="!tab.disabled && handleTabClick(tab.id)"
              class="nav-item"
            >
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

      <nav v-if="layoutMode === 'tabs'" class="layout-tabs">
        <div class="tabs-container">
          <button
            v-for="tab in tabs"
            :key="tab.id"
            :class="{
              'tab-button': true,
              active: modelValue === tab.id,
              disabled: tab.disabled,
            }"
            @click="!tab.disabled && handleTabClick(tab.id)"
          >
            <span class="tab-icon">{{ tab.icon }}</span>
            <span class="tab-label">{{ tab.label }}</span>
            <span v-if="tab.badge" class="tab-badge">{{ tab.badge }}</span>
          </button>
        </div>
      </nav>

      <main class="layout-main">
        <div class="main-content">
          <div v-if="showContentHeader" class="content-header">
            <h2>{{ getActiveTabLabel }}</h2>
            <div class="content-actions">
              <slot name="content-actions"></slot>
            </div>
          </div>
          <div class="content-wrapper">
            <slot :name="modelValue" :active-tab="modelValue">
              <slot></slot>
            </slot>
          </div>
        </div>
      </main>
    </div>
  </div>
</template>

<style scoped lang="scss">
.side-nav-layout {
  min-height: 600px;
  @include transition-color;
  background-color: var(--bg-light);
  color: var(--text-primary);
  @include flex-center;
  padding: $spacing-2xl 0;
}

.layout-wrapper {
  @include flex-column(0);
  min-height: 700px;
  width: 100%;
  max-width: 1200px;
  background-color: var(--bg-card);
  @include transition-bg;
  border: 1px solid var(--border-color);
  border-radius: $radius-xl;
  overflow: hidden;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.05);
}

// ── Header ────────────────────────────────────────────────
.layout-header {
  @include flex-between;
  padding: $spacing-lg $spacing-2xl;
  border-bottom: 1px solid var(--border-color);
  background-color: var(--bg-card);
  z-index: 10;
}

.header-left {
  @include inline-flex($spacing-lg);
  h1 {
    font-size: $fs-xl;
    font-weight: $fw-bold;
    margin: 0;
  }
}

.back-btn {
  @include inline-flex($spacing-sm);
  background: none;
  border: none;
  padding: $spacing-sm $spacing-md;
  border-radius: $radius-md;
  cursor: pointer;
  color: var(--text-secondary);
  transition: all $transition-base;
  &:hover {
    background-color: var(--hover-bg);
    color: var(--text-primary);
  }
}

.back-icon {
  font-size: 18px;
}
.back-text {
  font-size: 14px;
  font-weight: $fw-medium;
}

.header-right {
  @include inline-flex($spacing-md);
}

// ── Layout grid modes ─────────────────────────────────────
.side-nav-layout.sidebar .layout-wrapper {
  display: grid;
  grid-template-columns: 260px 1fr;
  grid-template-rows: auto 1fr;
  grid-template-areas: "header header" "sidebar main";
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

.side-nav-layout.tabs .layout-wrapper {
  display: grid;
  grid-template-rows: auto auto 1fr;
  grid-template-areas: "header" "tabs" "main";
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

// ── Sidebar ───────────────────────────────────────────────
.layout-sidebar {
  border-right: 1px solid var(--border-color);
  @include flex-column(0);
  overflow-y: auto;
  background-color: var(--bg-card);
}

.sidebar-nav ul {
  list-style: none;
  padding: 0;
  margin: 0;
}

.nav-item {
  @include inline-flex($spacing-md);
  padding: 14px $spacing-xl;
  cursor: pointer;
  transition: background-color $transition-base;
  border-left: 4px solid transparent;
  &:hover:not(.disabled) {
    background-color: var(--hover-bg);
  }
  &.active {
    background-color: var(--active-bg);
    border-left-color: var(--accent-color);
    font-weight: $fw-semibold;
  }
  &.disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
}

.nav-icon {
  font-size: 18px;
  width: $spacing-2xl;
  text-align: center;
}
.nav-label {
  font-size: 15px;
  flex: 1;
}

.nav-badge {
  background-color: var(--accent-color);
  color: white;
  font-size: $fs-xs;
  padding: $spacing-xs $spacing-sm;
  border-radius: $radius-lg;
  margin-left: $spacing-sm;
}

.sidebar-footer {
  margin-top: auto;
  padding: $spacing-xl;
  border-top: 1px solid var(--border-color);
}

// ── Tabs ──────────────────────────────────────────────────
.layout-tabs {
  border-bottom: 1px solid var(--border-color);
  background-color: var(--bg-card);
  overflow-x: auto;
}

.tabs-container {
  display: flex;
  padding: 0 $spacing-xl;
}

.tab-button {
  @include inline-flex($spacing-sm);
  padding: $spacing-lg $spacing-2xl;
  background: none;
  border: none;
  border-bottom: 3px solid transparent;
  cursor: pointer;
  transition: all $transition-base;
  color: var(--text-secondary);
  white-space: nowrap;
  font-size: 15px;
  &:hover:not(.disabled) {
    background-color: var(--hover-bg);
    color: inherit;
  }
  &.active {
    color: var(--accent-color);
    border-bottom-color: var(--accent-color);
    font-weight: $fw-semibold;
    background-color: var(--active-bg);
  }
  &.disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
}

.tab-icon {
  font-size: $fs-md;
}
.tab-badge {
  background-color: var(--accent-color);
  color: white;
  font-size: $fs-xs;
  padding: $spacing-xs $spacing-sm;
  border-radius: $radius-lg;
  margin-left: $spacing-sm;
}

// ── Main content ──────────────────────────────────────────
.layout-main {
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.main-content {
  flex: 1;
  overflow-y: auto;
  padding: $spacing-2xl;
}

.content-header {
  @include flex-between;
  margin-bottom: $spacing-2xl;
  padding-bottom: $spacing-lg;
  border-bottom: 1px solid var(--border-color);
  h2 {
    font-size: 22px;
    font-weight: $fw-bold;
    margin: 0;
  }
}

.content-actions {
  @include inline-flex($spacing-md);
}
.content-wrapper {
  min-height: 0;
}

// ── Responsive ────────────────────────────────────────────
@include tablet {
  .side-nav-layout.sidebar .layout-wrapper {
    grid-template-columns: 1fr;
    grid-template-areas: "header" "main";
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
      padding: 0 $spacing-md;
    }
    .tab-button {
      padding: $spacing-md $spacing-lg;
      font-size: 14px;
    }
    .tab-icon {
      margin-right: 6px;
    }
  }
  .main-content {
    padding: $spacing-lg;
  }
  .content-header {
    flex-direction: column;
    align-items: flex-start;
    gap: $spacing-md;
  }
  .content-actions {
    width: 100%;
    justify-content: flex-end;
  }
}
</style>
