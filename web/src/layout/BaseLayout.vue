<script setup lang="ts">
import { Collection, Download, Headset, List, MagicStick, Moon, Search, Sunny, UserFilled } from "@element-plus/icons-vue";
import type { InputInstance } from "element-plus";
import { computed, ref } from "vue";
import { useI18n } from "vue-i18n";
import { useRoute, useRouter } from "vue-router";
import GlobalPlayer from "@/components/player/GlobalPlayer.vue";
import { useKeyboardShortcuts } from "@/composables/useKeyboardShortcuts";
import { usePwaInstall } from "@/composables/usePwaInstall";
import { usePlayerStore } from "@/store/player";
import { useInstanceStore } from "@/store/instance";
import { useThemeStore } from "@/store/theme";
import { useUserStore } from "@/store/user";

const router = useRouter();
const route = useRoute();
const { t } = useI18n();
const userStore = useUserStore();
const themeStore = useThemeStore();
const playerStore = usePlayerStore();
const instanceStore = useInstanceStore();
const { canInstall, install } = usePwaInstall();

const searchQuery = ref("");
const searchInputRef = ref<InputInstance>();
const handleSearch = () => {
  if (searchQuery.value) {
    router.push({ name: "Home", query: { q: searchQuery.value } });
  }
};
const hideSearch = computed(() => route.name === "Login" || route.name === "Register");
const copyright = computed(() => t("common.copyright", { year: new Date().getFullYear() }));

const handleLogout = async () => {
  await userStore.logout();
  if (instanceStore.libraryRequiresAuth) {
    playerStore.clear();
    playerStore.clearRecent();
  }
  router.push("/");
};

useKeyboardShortcuts({
  focusSearch: () => searchInputRef.value?.focus(),
});
</script>

<template>
  <el-container class="app-shell" :class="{ 'has-player': playerStore.hasTrack }">
    <el-header>
      <div class="header-content container">
        <button class="logo" type="button" :aria-label="$t('common.app_name')" @click="router.push('/')">
          <el-icon :size="22"><Headset /></el-icon>
          <span class="logo-label">{{ $t("common.app_name") }}</span>
        </button>

        <nav v-if="!hideSearch" class="library-nav" :aria-label="$t('library.navigation')">
          <router-link :to="{ name: 'Artists' }" :aria-label="$t('library.artists')">
            <el-icon><UserFilled /></el-icon><span>{{ $t("library.artists") }}</span>
          </router-link>
          <router-link :to="{ name: 'Albums' }" :aria-label="$t('library.albums')">
            <el-icon><Collection /></el-icon><span>{{ $t("library.albums") }}</span>
          </router-link>
          <router-link
            v-if="instanceStore.capabilities.classification_enabled"
            :to="{ name: 'Presets' }"
            :aria-label="$t('classification.title')"
          >
            <el-icon><MagicStick /></el-icon><span>{{ $t("classification.title") }}</span>
          </router-link>
          <router-link v-if="userStore.isLoggedIn" :to="{ name: 'Playlists' }" :aria-label="$t('playlist.title')">
            <el-icon><List /></el-icon><span>{{ $t("playlist.title") }}</span>
          </router-link>
        </nav>

        <div class="search-bar" v-if="!hideSearch">
          <el-input
            ref="searchInputRef"
            v-model="searchQuery"
            :placeholder="$t('common.search')"
            class="header-search"
            :prefix-icon="Search"
            @keyup.enter="handleSearch"
          />
        </div>

        <div class="user-actions">
          <el-tooltip v-if="canInstall" :content="$t('common.install_app')" placement="bottom">
            <el-button circle :icon="Download" :aria-label="$t('common.install_app')" @click="install" />
          </el-tooltip>
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
            <el-button v-if="instanceStore.registrationOpen" plain @click="router.push('/register')">
              {{ $t("common.register") }}
            </el-button>
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
        <span class="footer-left"> {{ copyright }} </span>
        <span class="footer-right">
          <a href="#" target="_blank" rel="noopener">{{ $t("base.license") }}</a>
          <span class="dot">·</span>
          <a href="#" target="_blank" rel="noopener">{{ $t("base.repository") }}</a>
          <span class="dot">·</span>
          <a href="#" target="_blank" rel="noopener">{{ $t("base.maintainer") }}</a>
        </span>
      </div>
    </el-footer>

    <GlobalPlayer />
  </el-container>
</template>

<style scoped lang="scss">
.app-shell {
  min-height: 100vh;
}

.app-shell.has-player {
  padding-bottom: 96px;
}

.header-content {
  @include flex-between;
  height: 100%;
}

.logo {
  display: inline-flex;
  align-items: center;
  gap: $spacing-xs;
  flex-shrink: 0;
  padding: 0;
  border: 0;
  background: transparent;
  font-size: $fs-2xl;
  font-family: inherit;
  font-weight: $fw-bold;
  line-height: 1;
  cursor: pointer;
  color: #fff;
  max-width: 250px;
}

.logo-label {
  @include text-ellipsis;
}

.search-bar {
  flex-grow: 1;
  max-width: 500px;
  margin: 0 2rem;

  :deep(.el-input__wrapper) {
    background-color: rgba(255, 255, 255, 0.1);
    box-shadow: none;
    border: 1px solid rgba(255, 255, 255, 0.2);
  }
  :deep(.el-input__inner) {
    color: #fff;
    &::placeholder {
      color: rgba(255, 255, 255, 0.6);
    }
  }
}

.library-nav {
  display: flex;
  align-items: center;
  gap: $spacing-xs;
  margin-left: $spacing-lg;

  a {
    display: inline-flex;
    align-items: center;
    gap: $spacing-xs;
    padding: $spacing-xs $spacing-sm;
    border-radius: $radius-md;
    color: rgba(255, 255, 255, 0.82);
    text-decoration: none;

    &:hover,
    &.router-link-active {
      background: rgba(255, 255, 255, 0.14);
      color: #fff;
    }
  }
}

.user-actions {
  display: flex;
  align-items: center;
  gap: 1rem;
}

.username {
  color: #fff;
  font-weight: $fw-semibold;
  cursor: pointer;
}

@include mobile-xs {
  .logo {
    max-width: none;
  }
  .logo-label {
    display: none;
  }
  .search-bar {
    display: none;
  }
}
@include tablet {
  .logo {
    max-width: 120px;
    font-size: $fs-xl;
  }
  .search-bar {
    margin: 0 1rem;
    max-width: 300px;
  }

  .library-nav {
    margin-left: $spacing-sm;

    span {
      display: none;
    }
  }
}

.footer-content {
  @include flex-between;
  color: $color-text-muted;
  padding: $spacing-xl 0;
  font-size: $fs-base;
}

.footer-right a {
  color: $color-text-muted;
  text-decoration: none;
  &:hover {
    text-decoration: underline;
  }
}

.dot {
  margin: 0 6px;
}

@include mobile-xs {
  .footer-content {
    flex-direction: column;
    gap: 6px;
  }
}
</style>
