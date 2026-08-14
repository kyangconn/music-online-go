<script setup lang="ts">
import type { FormInstance, FormRules } from "element-plus";
import { ElMessage } from "element-plus";
import { ref, reactive } from "vue";
import { useI18n } from "vue-i18n";
import { useRouter } from "vue-router";
import type { LoginData } from "@/types/api";
import { useApiError } from "@/composables/useApiError";
import { useUserStore } from "@/store/user";
import { useInstanceStore } from "@/store/instance";
import request from "@/utils/request";

const router = useRouter();
const { t } = useI18n();
const { getErrorMessage } = useApiError();
const userStore = useUserStore();
const instanceStore = useInstanceStore();
const formRef = ref<FormInstance>();
const loading = ref(false);
const totpRequired = ref(false);

const loginForm = reactive({
  username: "",
  password: "",
  totp_code: "",
});

const rules = reactive<FormRules>({
  username: [{ required: true, message: t("auth.username_input"), trigger: "blur" }],
  password: [{ required: true, message: t("auth.password_input"), trigger: "blur" }],
  totp_code: [
    {
      validator: (_rule: unknown, value: string, callback: (error?: Error) => void) => {
        if (totpRequired.value && !value) {
          callback(new Error(t("auth.totp_code_required")));
        } else if (value && !/^\d{6}$/.test(value)) {
          callback(new Error(t("auth.totp_code_invalid")));
        } else {
          callback();
        }
      },
      trigger: "blur",
    },
  ],
});

const handleLogin = async (formEl: FormInstance | undefined) => {
  if (!formEl) return;
  await formEl.validate(async (valid) => {
    if (valid) {
      loading.value = true;
      try {
		const res = await request.post<LoginData>("/users/login", loginForm);
		userStore.setToken(res.data.access_token);
		userStore.setUser(res.data.user);
        ElMessage.success(t("common.login_successful"));
        const redirect = router.currentRoute.value.query.redirect;
        router.push(typeof redirect === "string" ? redirect : "/");
      } catch (error) {
        const message = getErrorMessage(error, t("auth.login_failed"));
        if (message === "TOTP code required" || message === "Invalid TOTP code") {
          totpRequired.value = true;
        }
        ElMessage.error(message);
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
          <h2>{{ $t("auth.login_welcome") }}</h2>
          <p>{{ $t("auth.login_subtitle") }}</p>
        </div>
      </template>

      <el-form ref="formRef" :model="loginForm" :rules="rules" label-position="top" size="large">
        <el-form-item :label="$t('auth.username_label')" prop="username">
          <el-input v-model="loginForm.username" :placeholder="$t('auth.username_placeholder')" />
        </el-form-item>

        <el-form-item :label="$t('auth.password_label')" prop="password">
          <el-input
            v-model="loginForm.password"
            type="password"
            :placeholder="$t('auth.password_placeholder')"
            show-password
            @keyup.enter="handleLogin(formRef)"
          />
        </el-form-item>

        <el-form-item v-if="totpRequired" :label="$t('auth.totp_code_label')" prop="totp_code">
          <el-input
            v-model="loginForm.totp_code"
            :placeholder="$t('auth.totp_code_placeholder')"
            maxlength="6"
            inputmode="numeric"
            @keyup.enter="handleLogin(formRef)"
          />
        </el-form-item>

        <el-form-item>
          <el-button type="primary" :loading="loading" class="w-full" @click="handleLogin(formRef)">
            {{ $t("auth.login_btn") }}
          </el-button>
        </el-form-item>
        <el-form-item>
          <el-button class="w-full" @click="router.push('/')">{{ $t("auth.back_home") }}</el-button>
        </el-form-item>
      </el-form>

      <div v-if="instanceStore.registrationOpen" class="auth-footer-link">
        <p>
          {{ $t("auth.no_account") }} <router-link to="/register">{{ $t("auth.register_now") }}</router-link>
        </p>
      </div>
    </el-card>
  </div>
</template>
