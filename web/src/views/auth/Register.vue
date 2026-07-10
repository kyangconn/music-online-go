<script setup lang="ts">
import type { FormInstance, FormRules } from "element-plus";
import { ElMessage } from "element-plus";
import { ref, reactive } from "vue";
import { useI18n } from "vue-i18n";
import { useRouter } from "vue-router";
import { useApiError } from "@/composables/useApiError";
import request from "@/utils/request";

const router = useRouter();
const { t } = useI18n();
const { getErrorMessage } = useApiError();
const formRef = ref<FormInstance>();
const loading = ref(false);

const registerForm = reactive({
  username: "",
  email: "",
  password: "",
  confirmPassword: "",
  full_name: "",
});

const validatePass2 = (_rule: unknown, value: string, callback: (error?: Error) => void) => {
  if (value === "") {
    callback(new Error(t("auth.confirm_password_required")));
  } else if (value !== registerForm.password) {
    callback(new Error(t("auth.passwords_not_match")));
  } else {
    callback();
  }
};

const rules = reactive<FormRules>({
  username: [{ required: true, message: t("auth.username_required"), trigger: "blur" }],
  email: [
    { required: true, message: t("auth.email_required"), trigger: "blur" },
    { type: "email", message: t("auth.email_invalid"), trigger: "blur" },
  ],
  password: [
    { required: true, message: t("auth.password_required"), trigger: "blur" },
    { min: 8, message: t("auth.password_min_length"), trigger: "blur" },
  ],
  confirmPassword: [
    { required: true, message: t("auth.confirm_password_required"), trigger: "blur" },
    { validator: validatePass2, trigger: "blur" },
  ],
  full_name: [{ required: true, message: t("auth.full_name_required"), trigger: "blur" }],
});

const handleRegister = async (formEl: FormInstance | undefined) => {
  if (!formEl) return;
  await formEl.validate(async (valid) => {
    if (valid) {
      loading.value = true;
      try {
        const { confirmPassword: _confirmPw, ...data } = registerForm;
        await request.post("/users/register", data);
        ElMessage.success(t("auth.register_success"));
        router.push("/login");
      } catch (error) {
        ElMessage.error(getErrorMessage(error, t("auth.register_failed")));
      } finally {
        loading.value = false;
      }
    }
  });
};
</script>

<template>
  <div class="auth-layout">
    <el-card class="auth-box">
      <template #header>
        <div class="card-header">
          <h2>{{ $t("auth.register_title") }}</h2>
          <p>{{ $t("auth.register_subtitle") }}</p>
        </div>
      </template>

      <el-form ref="formRef" :model="registerForm" :rules="rules" label-position="top" size="large">
        <el-form-item :label="$t('auth.username')" prop="username">
          <el-input v-model="registerForm.username" :placeholder="$t('auth.username_choose')" />
        </el-form-item>
        <el-form-item :label="$t('auth.full_name')" prop="full_name">
          <el-input v-model="registerForm.full_name" :placeholder="$t('auth.full_name_placeholder')" />
        </el-form-item>
        <el-form-item :label="$t('auth.email')" prop="email">
          <el-input v-model="registerForm.email" :placeholder="$t('auth.email_placeholder')" />
        </el-form-item>
        <el-form-item :label="$t('auth.password_label')" prop="password">
          <el-input
            v-model="registerForm.password"
            type="password"
            :placeholder="$t('auth.password_create')"
            show-password
          />
        </el-form-item>
        <el-form-item :label="$t('auth.confirm_password')" prop="confirmPassword">
          <el-input
            v-model="registerForm.confirmPassword"
            type="password"
            :placeholder="$t('auth.confirm_password_placeholder')"
            show-password
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="loading" class="w-full" @click="handleRegister(formRef)">
            {{ $t("auth.register_btn") }}
          </el-button>
        </el-form-item>
        <el-form-item>
          <el-button class="w-full" @click="router.push('/')">{{ $t("auth.back_home") }}</el-button>
        </el-form-item>
      </el-form>

      <div class="auth-footer-link">
        <p>
          {{ $t("auth.have_account") }} <router-link to="/login">{{ $t("auth.login_here") }}</router-link>
        </p>
      </div>
    </el-card>
  </div>
</template>
