<script setup lang="ts">
import { Plus, Search, Refresh } from "@element-plus/icons-vue";
import type { FormInstance, FormRules } from "element-plus";
import { ElMessage } from "element-plus";
import { computed, onMounted, reactive, ref } from "vue";
import { useI18n } from "vue-i18n";
import type { AdminCreateUserRequest, UserInfo } from "@/types/api";
import { useApiError } from "@/composables/useApiError";
import { usePaginatedFetch } from "@/composables/usePaginatedFetch";
import { useUserStore } from "@/store/user";
import request from "@/utils/request";
import { validateNewPassword } from "@/utils/password";

const { t } = useI18n();
const userStore = useUserStore();
const { handleError } = useApiError();
const query = ref("");
const createDialogVisible = ref(false);
const creating = ref(false);
const createFormRef = ref<FormInstance>();
const createForm = reactive<AdminCreateUserRequest>({
  username: "",
  email: "",
  password: "",
  full_name: "",
  role: "user",
});

const validatePassword = (_rule: unknown, value: string, callback: (error?: Error) => void) => {
  const validationError = validateNewPassword(value);
  if (validationError === "too_short") callback(new Error(t("auth.password_min_length")));
  else if (validationError === "too_long") callback(new Error(t("auth.password_max_bytes")));
  else callback();
};

const createRules = reactive<FormRules>({
  username: [{ required: true, message: t("auth.username_required"), trigger: "blur" }],
  email: [
    { required: true, message: t("auth.email_required"), trigger: "blur" },
    { type: "email", message: t("auth.email_invalid"), trigger: "blur" },
  ],
  password: [
    { required: true, message: t("auth.password_required"), trigger: "blur" },
    { validator: validatePassword, trigger: "blur" },
  ],
});

const { items: users, loading, total, currentPage, pageSize, fetch: fetchUsers, resetAndFetch } =
  usePaginatedFetch<UserInfo>("/users/admin/users", {
    errorMessageKey: "admin.load_users_failed",
    extraParams: computed(() => ({ q: query.value })),
  });

const handleSearch = () => {
  resetAndFetch();
};

const updateUserStatus = async (user: UserInfo) => {
  try {
    await request.put(`/users/admin/users/${user.id}/status`, { is_active: user.is_active });
    ElMessage.success(t("admin.user_updated"));
  } catch (e) {
    handleError(e, t("admin.user_update_failed"));
    fetchUsers();
  }
};

const updateUserRole = async (user: UserInfo) => {
  try {
    await request.put(`/users/admin/users/${user.id}/role`, { role: user.role });
    ElMessage.success(t("admin.user_updated"));
  } catch (e) {
    handleError(e, t("admin.user_update_failed"));
    fetchUsers();
  }
};

const resetCreateForm = () => {
  Object.assign(createForm, { username: "", email: "", password: "", full_name: "", role: "user" });
  createFormRef.value?.clearValidate();
};

const createUser = async () => {
  if (!createFormRef.value) return;
  await createFormRef.value.validate(async (valid) => {
    if (!valid) return;
    creating.value = true;
    try {
      await request.post<UserInfo>("/users/admin/users", createForm);
      ElMessage.success(t("admin.user_created"));
      createDialogVisible.value = false;
      resetCreateForm();
      resetAndFetch();
    } catch (error) {
      handleError(error, t("admin.user_create_failed"));
    } finally {
      creating.value = false;
    }
  });
};

onMounted(fetchUsers);
</script>

<template>
  <section class="admin-panel">
    <div class="admin-toolbar">
      <el-input
        v-model="query"
        :placeholder="$t('admin.search_users')"
        clearable
        class="search-input"
        @keyup.enter="handleSearch"
        @clear="handleSearch"
      >
        <template #prefix>
          <el-icon><Search /></el-icon>
        </template>
      </el-input>
      <el-button type="primary" :icon="Search" @click="handleSearch">{{ $t("admin.search") }}</el-button>
      <el-button :icon="Refresh" @click="fetchUsers">{{ $t("admin.refresh") }}</el-button>
      <el-button type="success" :icon="Plus" @click="createDialogVisible = true">
        {{ $t("admin.create_user") }}
      </el-button>
    </div>

    <el-table v-loading="loading" :data="users" size="small" class="admin-table">
      <el-table-column prop="username" :label="$t('auth.username')" min-width="140" show-overflow-tooltip />
      <el-table-column prop="email" :label="$t('auth.email')" min-width="200" show-overflow-tooltip />
      <el-table-column :label="$t('admin.role')" width="140">
        <template #default="{ row }">
          <el-select
            v-model="row.role"
            size="small"
            :disabled="row.id === userStore.user?.id"
            @change="updateUserRole(row)"
          >
            <el-option label="user" value="user" />
            <el-option label="admin" value="admin" />
          </el-select>
        </template>
      </el-table-column>
      <el-table-column :label="$t('admin.active')" width="120">
        <template #default="{ row }">
          <el-switch
            v-model="row.is_active"
            :disabled="row.id === userStore.user?.id"
            @change="updateUserStatus(row)"
          />
        </template>
      </el-table-column>
      <el-table-column prop="totp_enabled" :label="$t('admin.totp')" width="100">
        <template #default="{ row }">
          <el-tag :type="row.totp_enabled ? 'success' : 'info'" size="small">
            {{ row.totp_enabled ? $t("admin.enabled") : $t("admin.disabled") }}
          </el-tag>
        </template>
      </el-table-column>
    </el-table>

    <div class="admin-pagination">
      <el-pagination
        v-model:current-page="currentPage"
        background
        layout="prev, pager, next"
        :page-size="pageSize"
        :total="total"
        @current-change="fetchUsers"
      />
    </div>

    <el-dialog
      v-model="createDialogVisible"
      :title="$t('admin.create_user')"
      width="min(520px, 92vw)"
      @closed="resetCreateForm"
    >
      <el-form ref="createFormRef" :model="createForm" :rules="createRules" label-position="top">
        <el-form-item :label="$t('auth.username')" prop="username">
          <el-input v-model="createForm.username" autocomplete="off" />
        </el-form-item>
        <el-form-item :label="$t('auth.full_name')" prop="full_name">
          <el-input v-model="createForm.full_name" autocomplete="off" />
        </el-form-item>
        <el-form-item :label="$t('auth.email')" prop="email">
          <el-input v-model="createForm.email" autocomplete="off" />
        </el-form-item>
        <el-form-item :label="$t('auth.password_label')" prop="password">
          <el-input v-model="createForm.password" type="password" show-password autocomplete="new-password" />
        </el-form-item>
        <el-form-item :label="$t('admin.role')" prop="role">
          <el-select v-model="createForm.role">
            <el-option label="user" value="user" />
            <el-option label="admin" value="admin" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createDialogVisible = false">{{ $t("common.cancel") }}</el-button>
        <el-button type="primary" :loading="creating" @click="createUser">{{ $t("admin.create_user") }}</el-button>
      </template>
    </el-dialog>
  </section>
</template>

<style scoped lang="scss">
.admin-panel {
  width: 100%;
}

.admin-toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: $spacing-sm;
  margin-bottom: $spacing-lg;
}

.search-input {
  max-width: 320px;
}

.admin-table {
  width: 100%;
}

.admin-pagination {
  @include flex-center;
  margin-top: $spacing-lg;
}
</style>
