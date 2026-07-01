<script setup lang="ts">
import { Search, Refresh } from "@element-plus/icons-vue";
import { ElMessage } from "element-plus";
import { onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import type { UserInfo } from "@/types/api";
import { useUserStore } from "@/store/user";
import request from "@/utils/request";

interface UserListData {
  items: UserInfo[];
  total: number;
  page: number;
  size: number;
}

const { t } = useI18n();
const userStore = useUserStore();
const loading = ref(false);
const users = ref<UserInfo[]>([]);
const query = ref("");
const currentPage = ref(1);
const pageSize = ref(10);
const total = ref(0);

const fetchUsers = async () => {
  loading.value = true;
  try {
    const res = await request.get<UserListData>("/users/admin/users", {
      params: {
        q: query.value,
        page: currentPage.value,
        page_size: pageSize.value,
      },
    });
    users.value = res.data.items || [];
    total.value = res.data.total;
  } catch (_e) {
    ElMessage.error(t("admin.load_users_failed"));
  } finally {
    loading.value = false;
  }
};

const handleSearch = () => {
  currentPage.value = 1;
  fetchUsers();
};

const updateUserStatus = async (user: UserInfo) => {
  try {
    await request.put(`/users/admin/users/${user.id}/status`, { is_active: user.is_active });
    ElMessage.success(t("admin.user_updated"));
  } catch (_e) {
    ElMessage.error(t("admin.user_update_failed"));
    fetchUsers();
  }
};

const updateUserRole = async (user: UserInfo) => {
  try {
    await request.put(`/users/admin/users/${user.id}/role`, { role: user.role });
    ElMessage.success(t("admin.user_updated"));
  } catch (_e) {
    ElMessage.error(t("admin.user_update_failed"));
    fetchUsers();
  }
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
