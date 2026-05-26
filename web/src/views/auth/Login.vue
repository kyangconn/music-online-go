<script setup lang="ts">
import type { FormInstance, FormRules } from "element-plus"
import { ElMessage } from "element-plus"
import { ref, reactive } from "vue"
import { useRouter } from "vue-router"
import type { LoginData } from "@/types/api"
import { useUserStore } from "@/store/user"
import request from "@/utils/request"

const router = useRouter()
const userStore = useUserStore()
const formRef = ref<FormInstance>()
const loading = ref(false)

const loginForm = reactive({
  username: "",
  password: "",
})

const rules = reactive<FormRules>({
  username: [{ required: true, message: "Please input username", trigger: "blur" }],
  password: [{ required: true, message: "Please input password", trigger: "blur" }],
})

const handleLogin = async (formEl: FormInstance | undefined) => {
  if (!formEl) return
  await formEl.validate(async (valid) => {
    if (valid) {
      loading.value = true
      try {
        const res = await request.post<LoginData>("/users/login", loginForm)
        userStore.setToken(res.data.token)
        userStore.setUser(res.data.user)
        ElMessage.success("Login successful")
        router.push("/")
      } catch (_e) {
      } finally {
        loading.value = false
      }
    }
  })
}
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

        <el-form-item>
          <el-button type="primary" :loading="loading" class="w-full" @click="handleLogin(formRef)">
            {{ $t("auth.login_btn") }}
          </el-button>
        </el-form-item>
        <el-form-item>
          <el-button class="w-full" @click="router.push('/')">{{ $t("auth.back_home") }}</el-button>
        </el-form-item>
      </el-form>

      <div class="auth-footer-link">
        <p>
          {{ $t("auth.no_account") }} <router-link to="/register">{{ $t("auth.register_now") }}</router-link>
        </p>
      </div>
    </el-card>
  </div>
</template>
