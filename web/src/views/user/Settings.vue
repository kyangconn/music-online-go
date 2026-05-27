<script setup lang="ts">
import { ref, computed } from "vue";
import { useI18n } from "vue-i18n";
import { useRouter } from "vue-router";
import AdvancedSettings from "@/components/settings/AdvancedSettings.vue";
import GeneralSettings from "@/components/settings/GeneralSettings.vue";
import ProfileSettings from "@/components/settings/ProfileSettings.vue";
import SecuritySettings from "@/components/settings/SecuritySettings.vue";
import SideNavLayout, { type TabItem } from "@/layout/SideNavLayout.vue";

const router = useRouter();
const { t } = useI18n();

const goBack = () => router.back();

const tabs = computed<TabItem[]>(() => [
  { id: "general", label: t("settings.general") },
  { id: "profile", label: t("settings.profile") },
  { id: "security", label: t("settings.security") },
  { id: "advanced", label: t("settings.advanced") },
]);

const activeTab = ref("general");
const title = computed(() => t("settings.title"));
</script>

<template>
  <SideNavLayout v-model="activeTab" :title="title" :tabs="tabs" show-back-button @back="goBack">
    <template #general>
      <GeneralSettings />
    </template>

    <template #profile>
      <ProfileSettings @cancel="goBack" />
    </template>

    <template #security>
      <SecuritySettings />
    </template>

    <template #advanced>
      <AdvancedSettings />
    </template>
  </SideNavLayout>
</template>
