<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useUserStore } from '@/store/user'
import { ElMessage } from 'element-plus'
import QRCode from 'qrcode'

const { t } = useI18n()
const userStore = useUserStore()

const loading = ref(false)
const setupOpen = ref(false)
const secret = ref('')
const qrDataUrl = ref('')
const setupCode = ref('')
const disableCode = ref('')
const disableDialogVisible = ref(false)

const startSetup = async () => {
  loading.value = true
  try {
    const data = await userStore.setupTOTP()
    secret.value = data.secret
    qrDataUrl.value = await QRCode.toDataURL(data.qr_code_url)
    setupOpen.value = true
  } catch (_e) {
    ElMessage.error('Failed to setup TOTP')
  } finally {
    loading.value = false
  }
}

const handleEnable = async () => {
  if (setupCode.value.length !== 6) {
    ElMessage.warning(t('settings.totp_code_required'))
    return
  }
  loading.value = true
  try {
    await userStore.enableTOTP(setupCode.value)
    ElMessage.success(t('settings.totp_enabled_success'))
    setupOpen.value = false
    setupCode.value = ''
  } catch (_e) {
    ElMessage.error('Invalid code')
  } finally {
    loading.value = false
  }
}

const handleDisable = async () => {
  if (disableCode.value.length !== 6) {
    ElMessage.warning(t('settings.totp_code_required'))
    return
  }
  loading.value = true
  try {
    await userStore.disableTOTP(disableCode.value)
    ElMessage.success(t('settings.totp_disabled_success'))
    disableDialogVisible.value = false
    disableCode.value = ''
  } catch (_e) {
    ElMessage.error('Invalid code')
  } finally {
    loading.value = false
  }
}

const cancelSetup = () => {
  setupOpen.value = false
  setupCode.value = ''
}
</script>

<template>
  <div class="settings-section">
    <h3 class="section-title">{{ $t('settings.totp_title') }}</h3>

    <div class="setting-item">
      <div class="setting-info">
        <h4>{{ $t('settings.totp_title') }}</h4>
        <p v-if="userStore.user?.totp_enabled">{{ $t('settings.totp_enabled_desc') }}</p>
        <p v-else>{{ $t('settings.totp_disabled_desc') }}</p>
      </div>
      <div class="setting-control">
        <el-tag v-if="userStore.user?.totp_enabled" type="success" size="large">
          {{ $t('settings.totp_enabled') }}
        </el-tag>
        <el-button v-if="!userStore.user?.totp_enabled" type="primary" :loading="loading" @click="startSetup">
          {{ $t('settings.totp_setup') }}
        </el-button>
        <el-button v-else type="danger" plain @click="disableDialogVisible = true">
          {{ $t('settings.totp_disable') }}
        </el-button>
      </div>
    </div>

    <div v-if="setupOpen" class="totp-setup-panel">
      <p class="scan-hint">{{ $t('settings.totp_scan_hint') }}</p>
      <img v-if="qrDataUrl" :src="qrDataUrl" alt="TOTP QR Code" class="qr-image" />
      <p class="secret-text">Manual key: <code>{{ secret }}</code></p>

      <el-input
        v-model="setupCode"
        :placeholder="$t('settings.totp_code_placeholder')"
        maxlength="6"
        class="code-input"
        @keyup.enter="handleEnable"
      />
      <div class="totp-actions">
        <el-button @click="cancelSetup">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="loading" @click="handleEnable">
          {{ $t('settings.totp_verify_enable') }}
        </el-button>
      </div>
    </div>

    <el-dialog v-model="disableDialogVisible" :title="$t('settings.totp_disable')" width="400px">
      <p>{{ $t('settings.totp_disable_confirm') }}</p>
      <el-input
        v-model="disableCode"
        :placeholder="$t('settings.totp_code_placeholder')"
        maxlength="6"
        style="margin-top:16px"
        @keyup.enter="handleDisable"
      />
      <template #footer>
        <el-button @click="disableDialogVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="danger" :loading="loading" @click="handleDisable">
          {{ $t('settings.totp_confirm_disable') }}
        </el-button>
      </template>
    </el-dialog>
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

.totp-setup-panel {
  margin-top: 20px;
  padding: 24px;
  background: var(--bg-white);
  border-radius: 12px;
  border: 1px solid var(--border-color);
  text-align: center;
}

.scan-hint {
  margin: 0 0 16px;
  color: var(--text-secondary);
  font-size: 0.9rem;
}

.qr-image {
  width: 200px;
  height: 200px;
  border: 1px solid var(--border-color);
  border-radius: 8px;
  padding: 8px;
  background: #fff;
}

.secret-text {
  margin: 12px 0 16px;
  font-size: 0.85rem;
  color: var(--text-secondary);
}

.secret-text code {
  font-family: monospace;
  background: var(--hover-bg);
  padding: 2px 6px;
  border-radius: 4px;
  letter-spacing: 1px;
}

.code-input {
  max-width: 200px;
  margin: 0 auto 16px;
}

.totp-actions {
  display: flex;
  justify-content: center;
  gap: 12px;
}
</style>
