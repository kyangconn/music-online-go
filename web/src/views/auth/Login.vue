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

/** 处理登录表单提交 */
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
        // Error is handled by interceptor, but we can add specific handling here if needed
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
          <h2>Welcome Back</h2>
          <p>Login to access your music library</p>
        </div>
      </template>

      <el-form ref="formRef" :model="loginForm" :rules="rules" label-position="top" size="large">
        <el-form-item label="Username / Email" prop="username">
          <el-input v-model="loginForm.username" placeholder="Enter your username or email" />
        </el-form-item>

        <el-form-item label="Password" prop="password">
          <el-input
            v-model="loginForm.password"
            type="password"
            placeholder="Enter your password"
            show-password
            @keyup.enter="handleLogin(formRef)"
          />
        </el-form-item>

        <el-form-item>
          <el-button type="primary" :loading="loading" class="w-full" @click="handleLogin(formRef)"> Login </el-button>
        </el-form-item>
        <el-form-item>
          <el-button class="w-full" @click="router.push('/')"> Back to Home </el-button>
        </el-form-item>
      </el-form>

      <div class="auth-footer-link">
        <p>Don't have an account? <router-link to="/register">Register now</router-link></p>
      </div>
    </el-card>
  </div>
</template>
