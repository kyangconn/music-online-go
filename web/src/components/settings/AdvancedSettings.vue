<script setup lang="ts">
import { Delete } from "@element-plus/icons-vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { ref } from "vue";
import { useI18n } from "vue-i18n";
import { useRouter } from "vue-router";
import { useApiError } from "@/composables/useApiError";
import { useUserStore } from "@/store/user";

const directoryHandle = ref<FileSystemDirectoryHandle | null>(null);
const hasPermission = ref(false);
const requesting = ref(false);
const deletingAccount = ref(false);
const { t } = useI18n();
const router = useRouter();
const userStore = useUserStore();
const { handleError } = useApiError();

const requestDirectoryAccess = async () => {
  requesting.value = true;
  try {
    const handle = await (
      window as unknown as { showDirectoryPicker: (opts: { mode: string }) => Promise<FileSystemDirectoryHandle> }
    ).showDirectoryPicker({ mode: "read" });
    directoryHandle.value = handle;
    hasPermission.value = true;
    ElMessage.success(t("settings.local_access_granted"));
  } catch (error: unknown) {
    const err = error as DOMException;
    if (err.name === "AbortError") return;
    ElMessage.error(t("settings.local_access_error"));
  } finally {
    requesting.value = false;
  }
};

const revokeAccess = () => {
  directoryHandle.value = null;
  hasPermission.value = false;
  ElMessage.info(t("settings.local_access_revoked"));
};

const deleteAccount = async () => {
  try {
    const { value } = await ElMessageBox.prompt(t("settings.delete_account_warning"), t("settings.delete_account"), {
      cancelButtonText: t("common.cancel"),
      confirmButtonText: t("settings.delete_account_confirm"),
      inputPlaceholder: t("settings.delete_password_placeholder"),
      inputType: "password",
      inputValidator: (password) => Boolean(password) || t("settings.delete_password_required"),
      type: "warning",
    });
    deletingAccount.value = true;
    await userStore.deleteAccount(value);
    ElMessage.success(t("settings.delete_account_success"));
    await router.replace("/login");
  } catch (error) {
    if (error === "cancel" || error === "close") return;
    handleError(error, t("settings.delete_account_failed"));
  } finally {
    deletingAccount.value = false;
  }
};
</script>

<template>
  <div class="settings-section">
    <h3 class="section-title">{{ $t("settings.local_access_title") }}</h3>

    <div class="setting-item">
      <div class="setting-info">
        <h4>{{ $t("settings.local_access") }}</h4>
        <p>{{ $t("settings.local_access_desc") }}</p>
      </div>
      <el-button v-if="!hasPermission" type="primary" :loading="requesting" @click="requestDirectoryAccess">
        {{ $t("settings.local_access_request") }}
      </el-button>
      <el-button v-else type="danger" plain @click="revokeAccess">
        {{ $t("settings.local_access_revoke") }}
      </el-button>
    </div>

    <div v-if="hasPermission" class="setting-item">
      <div class="setting-info">
        <h4>{{ $t("settings.local_access_status") }}</h4>
        <p>{{ $t("settings.local_access_status_desc") }}</p>
      </div>
    </div>

    <div class="danger-zone">
      <h3 class="section-title">{{ $t("settings.account_lifecycle_title") }}</h3>
      <div class="setting-item">
        <div class="setting-info">
          <h4>{{ $t("settings.delete_account") }}</h4>
          <p>{{ $t("settings.delete_account_desc") }}</p>
        </div>
        <el-button type="danger" plain :icon="Delete" :loading="deletingAccount" @click="deleteAccount">
          {{ $t("settings.delete_account") }}
        </el-button>
      </div>
    </div>
  </div>
</template>

<style scoped lang="scss">
.danger-zone {
  margin-top: $spacing-3xl;
  padding-top: $spacing-2xl;
  border-top: 1px solid var(--border-color);
}

@include mobile {
  .danger-zone .setting-item {
    align-items: flex-start;
    flex-direction: column;
    gap: $spacing-lg;
  }
}
</style>
