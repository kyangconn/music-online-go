<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { useUserStore } from '@/store/user'
import request from '@/utils/request'
import { ElMessage } from 'element-plus'
import type { FormInstance, FormRules } from 'element-plus'

const router = useRouter()
const userStore = useUserStore()
const formRef = ref<FormInstance>()
const loading = ref(false)

const loginForm = reactive({
  username: '',
  password: ''
})

const rules = reactive<FormRules>({
  username: [{ required: true, message: 'Please input username', trigger: 'blur' }],
  password: [{ required: true, message: 'Please input password', trigger: 'blur' }]
})

const handleLogin = async (formEl: FormInstance | undefined) => {
  if (!formEl) return
  await formEl.validate(async (valid) => {
    if (valid) {
      loading.value = true
      try {
        const res: any = await request.post('/users/login', loginForm)
        userStore.setToken(res.data.token)
        userStore.setUser(res.data.user)
        ElMessage.success('Login successful')
        router.push('/')
      } catch (error: any) {
        // Error is handled by interceptor, but we can add specific handling here if needed
      } finally {
        loading.value = false
      }
    }
  })
}
</script>

<template>
  <div class="auth-container">
    <el-card class="auth-card">
      <template #header>
        <div class="card-header">
          <h2>Welcome Back</h2>
          <p>Login to access your music library</p>
        </div>
      </template>
      
      <el-form
        ref="formRef"
        :model="loginForm"
        :rules="rules"
        label-position="top"
        size="large"
      >
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
          <el-button type="primary" :loading="loading" class="w-100" @click="handleLogin(formRef)">
            Login
          </el-button>
        </el-form-item>
        <el-form-item>
          <el-button class="w-100" @click="router.push('/')">
            Back to Home
          </el-button>
        </el-form-item>
      </el-form>

      <div class="auth-footer">
        <p>Don't have an account? <router-link to="/register">Register now</router-link></p>
      </div>
    </el-card>
  </div>
</template>

<style scoped>
.auth-container {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: calc(100vh - 120px);
  background-color: var(--bg-light);
}

.auth-card {
  width: 100%;
  max-width: 400px;
  border-radius: 8px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}

.card-header h2 {
  margin: 0 0 8px;
  color: var(--text-dark);
  text-align: center;
}

.card-header p {
  margin: 0;
  color: var(--text-light);
  text-align: center;
  font-size: 0.9rem;
}

.w-100 {
  width: 100%;
}

.auth-footer {
  text-align: center;
  margin-top: 1rem;
  font-size: 0.9rem;
}

.auth-footer a {
  color: var(--accent-color);
  text-decoration: none;
}

.auth-footer a:hover {
  text-decoration: underline;
}
</style>
