<script setup lang="ts">
import { ElMessage, type FormInstance, type FormRules } from "element-plus";
import { ref, reactive, computed } from "vue";
import { useI18n } from "vue-i18n";
import { useUserStore } from "@/store/user";

const { t } = useI18n();
const userStore = useUserStore();

const loading = ref(false);
const formRef = ref<FormInstance>();

const updateForm = reactive({
  full_name: userStore.user?.full_name || "",
  email: userStore.user?.email || "",
  nickname: userStore.user?.nickname || "",
  bio: userStore.user?.bio || "",
  current_password: "",
  new_password: "",
  confirm_password: "",
});

const isChangingPassword = computed(() => updateForm.new_password.length > 0);

/** 验证确认密码 */
const validateConfirmPassword = (_rule: unknown, value: string, callback: (error?: Error) => void) => {
  if (isChangingPassword.value) {
    if (!value) {
      callback(new Error(t("settings.password_confirm_error")));
    } else if (value !== updateForm.new_password) {
      callback(new Error(t("settings.password_confirm_error")));
    } else {
      callback();
    }
  } else {
    callback();
  }
};

const rules = reactive<FormRules>({
  full_name: [{ required: true, message: "Please input full name", trigger: "blur" }],
  email: [
    { required: true, message: "Please input email", trigger: "blur" },
    { type: "email", message: "Please input correct email address", trigger: ["blur", "change"] },
  ],
  current_password: [
    {
      validator: (_rule: unknown, value: string, callback: (error?: Error) => void) => {
        if (isChangingPassword.value && !value) {
          callback(new Error(t("settings.password_required")));
        } else {
          callback();
        }
      },
      trigger: "blur",
    },
  ],
  new_password: [
    {
      validator: (_rule: unknown, value: string, callback: (error?: Error) => void) => {
        if (isChangingPassword.value && value.length < 6) {
          callback(new Error("Password must be at least 6 characters"));
        } else {
          callback();
        }
      },
      trigger: "blur",
    },
  ],
  confirm_password: [{ validator: validateConfirmPassword, trigger: "blur" }],
});

const emit = defineEmits<{
  (e: "cancel"): void;
}>();

/** 提交资料和密码修改 */
const handleSubmit = async (formEl: FormInstance | undefined) => {
  if (!formEl) return;
  try {
    const valid = await formEl.validate();
    if (!valid) return;
    loading.value = true;

    const profileData: Record<string, string> = {};
    if (updateForm.full_name !== (userStore.user?.full_name || "")) profileData.full_name = updateForm.full_name;
    if (updateForm.email !== (userStore.user?.email || "")) profileData.email = updateForm.email;
    if (updateForm.nickname !== (userStore.user?.nickname || "")) profileData.nickname = updateForm.nickname;
    if (updateForm.bio !== (userStore.user?.bio || "")) profileData.bio = updateForm.bio;

    if (Object.keys(profileData).length > 0) {
      await userStore.updateUser(profileData);
    }

    if (isChangingPassword.value) {
      await userStore.changePassword(updateForm.current_password, updateForm.new_password);
    }

    ElMessage.success(t("settings.save_success"));
    updateForm.current_password = "";
    updateForm.new_password = "";
    updateForm.confirm_password = "";
  } catch (error: unknown) {
    const msg = error instanceof Error ? error.message : t("settings.save_failed");
    ElMessage.error(msg);
  } finally {
    loading.value = false;
  }
};
</script>

<template>
  <el-form ref="formRef" :model="updateForm" :rules="rules" label-position="top" style="max-width: 800px">
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

    <el-divider />

    <el-row :gutter="20">
      <el-col :span="12">
        <el-form-item :label="$t('settings.password_new')" prop="new_password">
          <el-input
            v-model="updateForm.new_password"
            type="password"
            show-password
            :placeholder="$t('settings.password_new_desc')"
          />
        </el-form-item>
      </el-col>
      <el-col :span="12">
        <el-form-item :label="$t('settings.password_confirm')" prop="confirm_password">
          <el-input
            v-model="updateForm.confirm_password"
            type="password"
            show-password
            :disabled="!isChangingPassword"
          />
        </el-form-item>
      </el-col>
    </el-row>

    <div v-if="isChangingPassword" class="security-verify">
      <el-form-item :label="$t('settings.password_current')" prop="current_password">
        <el-input
          v-model="updateForm.current_password"
          type="password"
          show-password
          :placeholder="$t('settings.password_required')"
        />
      </el-form-item>
    </div>

    <el-form-item class="form-actions">
      <el-button @click="emit('cancel')">{{ $t("common.cancel") }}</el-button>
      <el-button type="primary" :loading="loading" @click="handleSubmit(formRef)">
        {{ $t("common.save") }}
      </el-button>
    </el-form-item>
  </el-form>
</template>

<style scoped lang="scss">
.security-verify {
  background: rgba(230, 162, 60, 0.05);
  padding: 20px;
  border-radius: 8px;
  border: 1px dashed #e6a23c;
  margin-bottom: 20px;
}

.form-actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}
</style>
