<script setup lang="ts">
import { ref, onMounted, computed } from "vue";
import { useI18n } from "vue-i18n";
import { useRouter } from "vue-router";
import type { SystemInfoData } from "@/types/api";
import DashboardOverview from "@/components/admin/DashboardOverview.vue";
import DatabaseInfo from "@/components/admin/DatabaseInfo.vue";
import MusicInfo from "@/components/admin/MusicInfo.vue";
import MusicManagement from "@/components/admin/MusicManagement.vue";
import MediaLibraryManagement from "@/components/admin/MediaLibraryManagement.vue";
import RuntimeInfo from "@/components/admin/RuntimeInfo.vue";
import ServerInfo from "@/components/admin/ServerInfo.vue";
import UserManagement from "@/components/admin/UserManagement.vue";
import { useApiError } from "@/composables/useApiError";
import SideNavLayout, { type TabItem } from "@/layout/SideNavLayout.vue";
import request from "@/utils/request";

const router = useRouter();
const { t } = useI18n();
const { handleError } = useApiError();

const loading = ref(false);
const error = ref(false);
const info = ref<SystemInfoData | null>(null);
const title = computed(() => t("admin.dashboard"));
const activeTab = ref("dashboard");

const tabs = computed<TabItem[]>(() => [
  { id: "dashboard", label: t("admin.dashboard") },
  { id: "server", label: t("admin.server") },
  { id: "runtime", label: t("admin.runtime") },
  { id: "database", label: t("admin.database") },
  { id: "music", label: t("admin.music") },
  { id: "user-management", label: t("admin.user_management") },
  { id: "music-management", label: t("admin.music_management") },
  { id: "media-library", label: t("admin.media_library") },
]);

/** 获取系统信息 */
const fetchInfo = async () => {
  loading.value = true;
  error.value = false;
  try {
    const res = await request.get<SystemInfoData>("/users/admin/system-info");
    info.value = res.data;
  } catch (e) {
    error.value = true;
    handleError(e, t("admin.load_system_info_failed"));
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
    class="admin-dashboard"
    v-model="activeTab"
    :title="title"
    :tabs="tabs"
    :show-content-header="false"
    show-back-button
    @back="goBack"
  >
    <template #dashboard>
      <DashboardOverview :loading="loading" :info="info" :error="error" @retry="fetchInfo" />
    </template>

    <template #server>
      <ServerInfo :loading="loading" :info="info" :error="error" @retry="fetchInfo" />
    </template>

    <template #runtime>
      <RuntimeInfo :loading="loading" :info="info" :error="error" @retry="fetchInfo" />
    </template>

    <template #database>
      <DatabaseInfo :loading="loading" :info="info" :error="error" @retry="fetchInfo" />
    </template>

    <template #music>
      <MusicInfo :loading="loading" :info="info" :error="error" @retry="fetchInfo" />
    </template>

    <template #user-management>
      <UserManagement />
    </template>

    <template #music-management>
      <MusicManagement />
    </template>

    <template #media-library>
      <MediaLibraryManagement />
    </template>
  </SideNavLayout>
</template>
