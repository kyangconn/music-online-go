<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import request from '@/utils/request'
import { ElMessage } from 'element-plus'
import type { FormInstance, FormRules } from 'element-plus'

const router = useRouter()
const formRef = ref<FormInstance>()
const loading = ref(false)

const registerForm = reactive({
  username: '',
  email: '',
  password: '',
  confirmPassword: '',
  full_name: ''
})

const validatePass2 = (_rule: any, value: any, callback: any) => {
  if (value === '') {
    callback(new Error('Please input the password again'))
  } else if (value !== registerForm.password) {
    callback(new Error("Two inputs don't match!"))
  } else {
    callback()
  }
}

const rules = reactive<FormRules>({
  username: [{ required: true, message: 'Please input username', trigger: 'blur' }],
  email: [
    { required: true, message: 'Please input email', trigger: 'blur' },
    { type: 'email', message: 'Please input correct email address', trigger: 'blur' }
  ],
  password: [{ required: true, message: 'Please input password', trigger: 'blur' }, { min: 6, message: 'Length should be at least 6', trigger: 'blur' }],
  confirmPassword: [{ validator: validatePass2, trigger: 'blur' }],
  full_name: [{ required: true, message: 'Please input full name', trigger: 'blur' }]
})

const handleRegister = async (formEl: FormInstance | undefined) => {
  if (!formEl) return
  await formEl.validate(async (valid) => {
    if (valid) {
      loading.value = true
      try {
        const { confirmPassword, ...data } = registerForm
        await request.post('/users/register', data)
        ElMessage.success('Registration successful! Please login.')
        router.push('/login')
      } catch (error: any) {
        // Error handled by interceptor
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
          <h2>Create Account</h2>
          <p>Join our music community today</p>
        </div>
      </template>

      <el-form ref="formRef" :model="registerForm" :rules="rules" label-position="top" size="large">
        <el-form-item label="Username" prop="username">
          <el-input v-model="registerForm.username" placeholder="Choose a username" />
        </el-form-item>

        <el-form-item label="Full Name" prop="full_name">
          <el-input v-model="registerForm.full_name" placeholder="Your full name" />
        </el-form-item>

        <el-form-item label="Email" prop="email">
          <el-input v-model="registerForm.email" placeholder="Your email address" />
        </el-form-item>

        <el-form-item label="Password" prop="password">
          <el-input v-model="registerForm.password" type="password" placeholder="Create a password" show-password />
        </el-form-item>

        <el-form-item label="Confirm Password" prop="confirmPassword">
          <el-input v-model="registerForm.confirmPassword" type="password" placeholder="Confirm your password"
            show-password />
        </el-form-item>

        <el-form-item>
          <el-button type="primary" :loading="loading" class="w-100" @click="handleRegister(formRef)">
            Register
          </el-button>
        </el-form-item>
        <el-form-item>
          <el-button class="w-100" @click="router.push('/')">
            Back to Home
          </el-button>
        </el-form-item>
      </el-form>

      <div class="auth-footer">
        <p>Already have an account? <router-link to="/login">Login here</router-link></p>
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
  padding: 2rem 0;
}

.auth-card {
  width: 100%;
  max-width: 450px;
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
