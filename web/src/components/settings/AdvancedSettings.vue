<script setup lang="ts">
import { ref } from 'vue';
import { ElMessage } from 'element-plus';
import { useI18n } from 'vue-i18n';

const directoryHandle = ref<any>(null);
const hasPermission = ref(false);
const requesting = ref(false);
const { t } = useI18n();

const requestDirectoryAccess = async () => {
  requesting.value = true
  try {
    const handle = await (window as any).showDirectoryPicker({ mode: 'read' })
    directoryHandle.value = handle
    hasPermission.value = true
    ElMessage.success(t('settings.local_access_granted'))
  } catch (error: unknown) {
    const err = error as DOMException
    if (err.name === 'AbortError') return
    ElMessage.error(t('settings.local_access_error'))
  } finally {
    requesting.value = false
  }
};

const revokeAccess = () => {
  directoryHandle.value = null
  hasPermission.value = false
  ElMessage.info(t('settings.local_access_revoked'))
};
</script>

<template>
  <div class="settings-section">
    <h3 class="section-title">{{ $t('settings.local_access_title') }}</h3>

    <div class="setting-item">
      <div class="setting-info">
        <h4>{{ $t('settings.local_access') }}</h4>
        <p>{{ $t('settings.local_access_desc') }}</p>
      </div>
      <el-button v-if="!hasPermission" type="primary" :loading="requesting" @click="requestDirectoryAccess">
        {{ $t('settings.local_access_request') }}
      </el-button>
      <el-button v-else type="danger" plain @click="revokeAccess">
        {{ $t('settings.local_access_revoke') }}
      </el-button>
    </div>

    <div v-if="hasPermission" class="setting-item">
      <div class="setting-info">
        <h4>{{ $t('settings.local_access_status') }}</h4>
        <p>{{ $t('settings.local_access_status_desc') }}</p>
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

.setting-info h4 {
  font-size: 16px;
  margin-bottom: 4px;
}

.setting-info p {
  font-size: 14px;
  color: var(--text-secondary);
}
</style>
