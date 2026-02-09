<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useUserStore } from '@/store/user'
import { Search } from '@element-plus/icons-vue'

const router = useRouter()
const userStore = useUserStore()

const searchQuery = ref('')
const handleSearch = () => {
  if (searchQuery.value) {
    router.push({ name: 'Home', query: { q: searchQuery.value } })
  }
}

const handleLogout = () => {
  userStore.logout()
  router.push('/login')
}
</script>

<template>
  <el-container>
    <el-header>
      <div class="header-content container">
        <div class="logo" @click="router.push('/')">
          🎵 Music Online
        </div>
        
        <div class="search-bar">
          <el-input
            v-model="searchQuery"
            placeholder="Search music..."
            class="w-50 m-2"
            :prefix-icon="Search"
            @keyup.enter="handleSearch"
          />
        </div>

        <div class="user-actions">
          <template v-if="userStore.isLoggedIn">
            <el-dropdown>
              <span class="el-dropdown-link text-white">
                <el-avatar :size="32" :src="userStore.user?.avatar_url || 'https://cube.elemecdn.com/3/7c/3ea6beec64369c2642b92c6726f1epng.png'" />
                <span class="username">{{ userStore.user?.username }}</span>
              </span>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item @click="router.push('/profile')">Profile</el-dropdown-item>
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
        <p>&copy; 2026 Music Online. All rights reserved.</p>
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
  margin-left: 8px;
  color: #fff;
  font-weight: 500;
  cursor: pointer;
}

.footer-content {
  text-align: center;
  color: #999;
  padding: 20px 0;
}

:deep(.el-input__wrapper) {
  background-color: rgba(255, 255, 255, 0.1);
  box-shadow: none;
  border: 1px solid rgba(255, 255, 255, 0.2);
}

:deep(.el-input__inner) {
  color: #fff;
}

:deep(.el-input__inner::placeholder) {
  color: rgba(255, 255, 255, 0.6);
}
</style>
