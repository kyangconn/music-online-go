<script setup lang="ts">
import { ref, computed, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useUserStore } from '@/store/user'
import { useThemeStore } from '@/store/theme'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import SideNavLayout, { type TabItem } from '@/layout/SideNavLayout.vue'

const router = useRouter()
const { t } = useI18n()
const userStore = useUserStore()
const themeStore = useThemeStore()

const goBack = () => {
  router.back()
}

// 布局模式切换
const layoutMode = ref<'sidebar' | 'tabs'>('sidebar')

// 标签页数据
const tabs = computed<TabItem[]>(() => [
  { id: 'general', label: t('settings.general') },
  { id: 'profile', label: t('settings.profile') },
  { id: 'privacy', label: t('settings.privacy') },
  { id: 'notifications', label: t('settings.notifications'), badge: 3 },
  { id: 'advanced', label: t('settings.advanced') }
])

const activeTab = ref('general')
const title = computed(() => t('settings.title'))
const loading = ref(false)
const formRef = ref<FormInstance>()

const handleTabChange = (_tabId: string) => {}

const updateForm = reactive({
  full_name: userStore.user?.full_name || '',
  email: userStore.user?.email || '',
  nickname: userStore.user?.nickname || '',
  bio: userStore.user?.bio || '',
  current_password: '',
  new_password: '',
  confirm_password: '',
})

const isChangingPassword = computed(() => updateForm.new_password.length > 0)

const validateConfirmPassword = (_rule: any, value: string, callback: any) => {
  if (isChangingPassword.value) {
    if (!value) {
      callback(new Error(t('settings.password_confirm_error')))
    } else if (value !== updateForm.new_password) {
      callback(new Error(t('settings.password_confirm_error')))
    } else {
      callback()
    }
  } else {
    callback()
  }
}

const rules = reactive<FormRules>({
  full_name: [{ required: true, message: 'Please input full name', trigger: 'blur' }],
  email: [
    { required: true, message: 'Please input email', trigger: 'blur' },
    { type: 'email', message: 'Please input correct email address', trigger: ['blur', 'change'] },
  ],
  current_password: [
    {
      validator: (_rule: any, value: string, callback: any) => {
        if (isChangingPassword.value && !value) {
          callback(new Error(t('settings.password_required')))
        } else {
          callback()
        }
      },
      trigger: 'blur',
    },
  ],
  new_password: [
    {
      validator: (_rule: any, value: string, callback: any) => {
        if (isChangingPassword.value && value.length < 6) {
          callback(new Error('Password must be at least 6 characters'))
        } else {
          callback()
        }
      },
      trigger: 'blur',
    },
  ],
  confirm_password: [{ validator: validateConfirmPassword, trigger: 'blur' }],
})

const handleUpdateForm = async (formEl: FormInstance | undefined) => {
  if (!formEl) return
  try {
    const valid = await formEl.validate()
    if (!valid) return
    loading.value = true

    const profileData: Record<string, string> = {}
    if (updateForm.full_name !== (userStore.user?.full_name || ''))
      profileData.full_name = updateForm.full_name
    if (updateForm.email !== (userStore.user?.email || ''))
      profileData.email = updateForm.email
    if (updateForm.nickname !== (userStore.user?.nickname || ''))
      profileData.nickname = updateForm.nickname
    if (updateForm.bio !== (userStore.user?.bio || ''))
      profileData.bio = updateForm.bio

    if (Object.keys(profileData).length > 0) {
      await userStore.updateUser(profileData)
    }

    if (isChangingPassword.value) {
      await userStore.changePassword(updateForm.current_password, updateForm.new_password)
    }

    ElMessage.success(t('settings.save_success'))
    updateForm.current_password = ''
    updateForm.new_password = ''
    updateForm.confirm_password = ''
  } catch (error: any) {
    ElMessage.error(error?.response?.data?.error || error.message || t('settings.save_failed'))
  } finally {
    loading.value = false
  }
}

// 本地文件访问设置
const directoryHandle = ref<any>(null)
const hasPermission = ref(false)
const requesting = ref(false)

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
}

const revokeAccess = () => {
  directoryHandle.value = null
  hasPermission.value = false
  ElMessage.info(t('settings.local_access_revoked'))
}
</script>

<template>
  <SideNavLayout v-model="activeTab" :title="title" :tabs="tabs" :layout-mode="layoutMode" show-back-button
    @tab-change="handleTabChange" @back="goBack">
    <template #general>
      <div class="settings-section">
        <h3 class="section-title">{{ $t('settings.general') }}</h3>

        <div class="setting-item">
          <div class="setting-info">
            <h4>{{ $t('settings.auto_sync') }}</h4>
            <p>{{ $t('settings.auto_sync_desc') }}</p>
          </div>
          <div class="setting-control">
            <el-switch v-model="themeStore.autoSync" />
          </div>
        </div>

        <div class="setting-item">
          <div class="setting-info">
            <h4>{{ $t('settings.theme_label') }}</h4>
            <p>{{ $t('settings.theme_desc') }}</p>
          </div>
          <div class="setting-control">
            <button class="theme-toggle-inline" @click="themeStore.toggleDarkMode" :disabled="themeStore.autoSync">
              <span v-if="themeStore.isDark">🌙 {{ $t('settings.dark') }}</span>
              <span v-else>☀️ {{ $t('settings.light') }}</span>
            </button>
          </div>
        </div>
      </div>
    </template>

    <template #profile>
      <div class="profile-section">
        <el-form ref="formRef" :model="updateForm" :rules="rules" label-position="top" class="profile-form-el">
          <el-row :gutter="20">
            <el-col :span="12">
              <el-form-item :label="$t('settings.name')" prop="nickname">
                <el-input v-model="updateForm.nickname" :placeholder="userStore.user?.username" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item :label="$t('settings.full_name')" prop="full_name">
                <el-input v-model="updateForm.full_name" />
              </el-form-item>
            </el-col>
          </el-row>

          <el-form-item :label="$t('settings.email')" prop="email">
            <el-input v-model="updateForm.email" />
          </el-form-item>

          <div class="form-divider"></div>

          <el-row :gutter="20">
            <el-col :span="12">
              <el-form-item :label="$t('settings.password_new')" prop="new_password">
                <el-input v-model="updateForm.new_password" type="password" show-password placeholder="Leave blank to keep current" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item :label="$t('settings.password_confirm')" prop="confirm_password">
                <el-input v-model="updateForm.confirm_password" type="password" show-password :disabled="!isChangingPassword" />
              </el-form-item>
            </el-col>
          </el-row>

          <div v-if="isChangingPassword" class="security-verify">
            <el-form-item :label="$t('settings.password_current')" prop="current_password">
              <el-input v-model="updateForm.current_password" type="password" show-password
                :placeholder="$t('settings.password_required')" />
            </el-form-item>
          </div>

          <div class="form-actions-el">
            <el-button @click="goBack">{{ $t('common.cancel') }}</el-button>
            <el-button type="primary" :loading="loading" @click="handleUpdateForm(formRef)">
              {{ $t('common.save') }}
            </el-button>
          </div>
        </el-form>
      </div>
    </template>

    <template #advanced>
      <div class="settings-section">
        <h3 class="section-title">{{ $t('settings.local_access_title') }}</h3>

        <div class="setting-item">
          <div class="setting-info">
            <h4>{{ $t('settings.local_access') }}</h4>
            <p>{{ $t('settings.local_access_desc') }}</p>
          </div>
          <div class="setting-control">
            <el-button v-if="!hasPermission" type="primary" :loading="requesting" @click="requestDirectoryAccess">
              {{ $t('settings.local_access_request') }}
            </el-button>
            <el-button v-else type="danger" plain @click="revokeAccess">
              {{ $t('settings.local_access_revoke') }}
            </el-button>
          </div>
        </div>

        <div v-if="hasPermission" class="setting-item">
          <div class="setting-info">
            <h4>{{ $t('settings.local_access_status') }}</h4>
            <p>{{ $t('settings.local_access_status_desc') }}</p>
          </div>
        </div>
      </div>
    </template>
  </SideNavLayout>
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

.theme-toggle-inline {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 16px;
  border-radius: 8px;
  border: 1px solid var(--border-color);
  background: var(--bg-card);
  cursor: pointer;
  transition: all 0.2s;
}

.theme-toggle-inline:hover:not(:disabled) {
  background: var(--hover-bg);
}

.theme-toggle-inline:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.profile-section {
  max-width: 800px;
}

.profile-form-el {
  margin-top: 8px;
}

.form-divider {
  height: 1px;
  background: var(--border-color);
  margin: 24px 0;
}

.security-verify {
  background: rgba(230, 162, 60, 0.05);
  padding: 20px;
  border-radius: 8px;
  border: 1px dashed #e6a23c;
  margin: 24px 0;
}

.form-actions-el {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  margin-top: 32px;
}

.actions-group {
  display: flex;
  gap: 12px;
}
</style>
