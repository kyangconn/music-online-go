<script setup lang="ts">
import { ElMessage } from "element-plus";
import { ref, onMounted, computed } from "vue";
import { useI18n } from "vue-i18n";
import { useRouter } from "vue-router";
import type { SystemInfoData } from "@/types/api";
import DashboardOverview from "@/components/admin/DashboardOverview.vue";
import DatabaseInfo from "@/components/admin/DatabaseInfo.vue";
import MusicInfo from "@/components/admin/MusicInfo.vue";
import RuntimeInfo from "@/components/admin/RuntimeInfo.vue";
import ServerInfo from "@/components/admin/ServerInfo.vue";
import SideNavLayout, { type TabItem } from "@/layout/SideNavLayout.vue";
import request from "@/utils/request";

const router = useRouter();
const { t } = useI18n();

const loading = ref(false);
const info = ref<SystemInfoData | null>(null);
const title = computed(() => t("admin.dashboard"));
const activeTab = ref("dashboard");

const tabs = computed<TabItem[]>(() => [
  { id: "dashboard", label: t("admin.dashboard") },
  { id: "server", label: t("admin.server") },
  { id: "runtime", label: t("admin.runtime") },
  { id: "database", label: t("admin.database") },
  { id: "music", label: t("admin.music") },
]);

/** 获取系统信息 */
const fetchInfo = async () => {
  loading.value = true;
  try {
    const res = await request.get<SystemInfoData>("/users/admin/system-info");
    info.value = res.data;
  } catch (_e) {
    ElMessage.error("Failed to load system info");
  } finally {
    loading.value = false;
  }
};

/** 返回上一页 */
const goBack = () => router.back();

onMounted(fetchInfo);
</script>

<template>
  <SideNavLayout
    v-model="activeTab"
    :title="title"
    :tabs="tabs"
    :show-content-header="false"
    show-back-button
    @back="goBack"
  >
    <template #dashboard>
      <DashboardOverview :loading="loading" :info="info!" />
    </template>

    <template #server>
      <ServerInfo :loading="loading" :info="info!" />
    </template>

    <template #runtime>
      <RuntimeInfo :loading="loading" :info="info!" />
    </template>

    <template #database>
      <DatabaseInfo :loading="loading" :info="info!" />
    </template>

    <template #music>
      <MusicInfo :loading="loading" :info="info!" />
    </template>
  </SideNavLayout>
</template>
