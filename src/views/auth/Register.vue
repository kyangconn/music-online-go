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
  confirmPassword: [
    { required: true, message: 'Please confirm your password', trigger: 'blur' },
    { validator: validatePass2, trigger: 'blur' },
  ],
  full_name: [{ required: true, message: 'Please input full name', trigger: 'blur' }]
})

const handleRegister = async (formEl: FormInstance | undefined) => {
  if (!formEl) return
  await formEl.validate(async (valid) => {
    if (valid) {
      loading.value = true
      try {
        const { confirmPassword: _confirmPw, ...data } = registerForm
        await request.post('/users/register', data)
        ElMessage.success('Registration successful! Please login.')
        router.push('/login')
      } catch (_e) {
        // Error handled by interceptor
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
          <el-button type="primary" :loading="loading" class="w-full" @click="handleRegister(formRef)">
            Register
          </el-button>
        </el-form-item>
        <el-form-item>
          <el-button class="w-full" @click="router.push('/')">
            Back to Home
          </el-button>
        </el-form-item>
      </el-form>

      <div class="auth-footer-link">
        <p>Already have an account? <router-link to="/login">Login here</router-link></p>
      </div>
    </el-card>
  </div>
</template>
