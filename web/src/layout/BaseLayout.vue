<script setup lang="ts">
import { Search, Moon, Sunny, Headset } from "@element-plus/icons-vue"
import { ref, computed } from "vue"
import { useRouter, useRoute } from "vue-router"
import { useThemeStore } from "@/store/theme"
import { useUserStore } from "@/store/user"

const router = useRouter()
const route = useRoute()
const userStore = useUserStore()
const themeStore = useThemeStore()

const searchQuery = ref("")
const handleSearch = () => {
  if (searchQuery.value) {
    router.push({ name: "Home", query: { q: searchQuery.value } })
  }
}
const hideSearch = computed(() => route.name === "Login" || route.name === "Register")

const handleLogout = () => {
  userStore.logout()
  router.push("/")
}
</script>

<template>
  <el-container>
    <el-header>
      <div class="header-content container">
        <div class="logo" @click="router.push('/')">
          <el-icon :size="22"><Headset /></el-icon>
          Music Online
        </div>

        <div class="search-bar" v-if="!hideSearch">
          <el-input
            v-model="searchQuery"
            :placeholder="$t('common.search')"
            class="header-search"
            :prefix-icon="Search"
            @keyup.enter="handleSearch"
          />
        </div>

        <div class="user-actions">
          <el-button circle @click="themeStore.toggleDarkMode" :icon="themeStore.isDark ? Sunny : Moon" />
          <template v-if="userStore.isLoggedIn">
            <el-dropdown>
              <span class="el-dropdown-link text-white username-only">
                <span class="username">{{ userStore.user?.username || $t("common.profile") }}</span>
              </span>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item @click="router.push('/profile')">{{ $t("common.profile") }}</el-dropdown-item>
                  <el-dropdown-item @click="router.push('/music/add')">{{ $t("common.upload") }}</el-dropdown-item>
                  <el-dropdown-item @click="router.push('/settings')">{{ $t("common.settings") }}</el-dropdown-item>
                  <el-dropdown-item v-if="userStore.isAdmin" @click="router.push('/admin')">{{
                    $t("common.admin")
                  }}</el-dropdown-item>
                  <el-dropdown-item divided @click="handleLogout">{{ $t("common.logout") }}</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </template>
          <template v-else>
            <el-button type="primary" plain @click="router.push('/login')">{{ $t("common.login") }}</el-button>
            <el-button plain @click="router.push('/register')">{{ $t("common.register") }}</el-button>
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
        <span class="footer-left"> © 2026 Music Online </span>
        <span class="footer-right">
          <a href="#" target="_blank" rel="noopener">{{ $t("base.license") }}</a>
          <span class="dot">·</span>
          <a href="#" target="_blank" rel="noopener">{{ $t("base.repository") }}</a>
          <span class="dot">·</span>
          <a href="#" target="_blank" rel="noopener">{{ $t("base.maintainer") }}</a>
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
