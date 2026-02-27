<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useUserStore } from '@/store/user'
import { Search, Moon, Sunny } from '@element-plus/icons-vue'

const router = useRouter()
const route = useRoute()
const userStore = useUserStore()

const searchQuery = ref('')
const handleSearch = () => {
  if (searchQuery.value) {
    router.push({ name: 'Home', query: { q: searchQuery.value } })
  }
}
const hideSearch = computed(() => route.name === 'Login' || route.name === 'Register')

const handleLogout = () => {
  userStore.logout()
  router.push('/login')
}

const isDark = ref(false)
const applyTheme = () => {
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}
onMounted(() => {
  const saved = localStorage.getItem('theme')
  isDark.value = saved === 'dark'
  applyTheme()
})
const toggleTheme = () => {
  isDark.value = !isDark.value
  applyTheme()
}
</script>

<template>
  <el-container>
    <el-header>
      <div class="header-content container">
        <div class="logo" @click="router.push('/')">
          🎵 Music Online
        </div>
        
        <div class="search-bar" v-if="!hideSearch">
          <el-input
            v-model="searchQuery"
            placeholder="Search music..."
            class="header-search"
            :prefix-icon="Search"
            @keyup.enter="handleSearch"
          />
        </div>

        <div class="user-actions">
          <el-button circle @click="toggleTheme" :icon="isDark ? Sunny : Moon" />
          <template v-if="userStore.isLoggedIn">
            <el-dropdown>
              <span class="el-dropdown-link text-white username-only">
                <span class="username">{{ userStore.user?.username || 'User' }}</span>
              </span>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item @click="router.push('/profile')">Profile</el-dropdown-item>
                  <el-dropdown-item @click="router.push('/music/add')">Upload Music</el-dropdown-item>
                  <el-dropdown-item v-if="userStore.isAdmin" @click="router.push('/admin')">Admin</el-dropdown-item>
                  <el-dropdown-item divided @click="handleLogout">Logout</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </template>
          <template v-else>
            <el-button type="primary" plain @click="router.push('/login')">Login</el-button>
            <el-button plain @click="router.push('/register')">Register</el-button>
          </template>
        </div>
      </div>
    </el-header>

    <el-main>
      <div class="container">
        <router-view />
      </div>
    </el-main>

    <el-footer>
      <div class="footer-content container">
        <span class="footer-left">
          © 2026 Music Online
        </span>
        <span class="footer-right">
          <a href="#" target="_blank" rel="noopener">License</a>
          <span class="dot">·</span>
          <a href="#" target="_blank" rel="noopener">Repository</a>
          <span class="dot">·</span>
          <a href="#" target="_blank" rel="noopener">Maintainers</a>
        </span>
      </div>
    </el-footer>
  </el-container>
</template>

<style scoped>
.header-content {
  display: flex;
  justify-content: space-between;
  align-items: center;
  height: 100%;
}

.logo {
  font-size: 1.5rem;
  font-weight: bold;
  cursor: pointer;
  color: #fff;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 250px;
}

.search-bar {
  flex-grow: 1;
  max-width: 500px;
  margin: 0 2rem;
}

.user-actions {
  display: flex;
  align-items: center;
  gap: 1rem;
}

.username {
  color: #fff;
  font-weight: 600;
  cursor: pointer;
}

.search-bar :deep(.el-input__wrapper) {
  background-color: rgba(255, 255, 255, 0.1);
  box-shadow: none;
  border: 1px solid rgba(255, 255, 255, 0.2);
}

.search-bar :deep(.el-input__inner) {
  color: #fff;
}

.search-bar :deep(.el-input__inner::placeholder) {
  color: rgba(255, 255, 255, 0.6);
}

@media (max-width: 768px) {
  .logo {
    max-width: 120px;
    font-size: 1.2rem;
  }
  .search-bar {
    margin: 0 1rem;
    max-width: 300px;
  }
}

@media (max-width: 480px) {
  .logo {
    max-width: 100px;
    font-size: 1rem;
  }
  .search-bar {
    display: none;
  }
}

.footer-content {
  display: flex;
  justify-content: space-between;
  align-items: center;
  color: #999;
  padding: 20px 0;
  font-size: 0.9rem;
}

.footer-right a {
  color: #999;
  text-decoration: none;
}

.footer-right a:hover {
  text-decoration: underline;
}

.dot {
  margin: 0 6px;
}

@media (max-width: 480px) {
  .footer-content {
    flex-direction: column;
    gap: 6px;
  }
}
</style>
