import { defineStore } from "pinia";
import { ref } from "vue";

/**
 * 主题状态管理存储
 * 管理应用的主题模式（深色/浅色）和系统主题自动同步功能
 */
export const useThemeStore = defineStore("theme", () => {
  /** 当前是否为深色模式 */
  const isDark = ref(false);
  /** 是否自动同步系统主题 */
  const autoSync = ref(true);
  let mediaQuery: MediaQueryList | null = null;
  let initialized = false;

  /**
   * 设置深色模式
   * @param value - true 为深色模式，false 为浅色模式
   */
  const setDarkMode = (value: boolean) => {
    isDark.value = value;
    try {
      if (value) {
        document.documentElement.classList.add("dark");
        localStorage.setItem("theme", "dark");
      } else {
        document.documentElement.classList.remove("dark");
        localStorage.setItem("theme", "light");
      }
    } catch (error) {
      console.error("Failed to set theme:", error);
      // 即使存储失败，也应用主题到DOM，但可能无法持久化
    }
  };

  /**
   * 设置自动同步系统主题功能
   * @param value - true 启用自动同步，false 禁用自动同步
   */
  const setAutoSync = (value: boolean) => {
    autoSync.value = value;
    try {
      localStorage.setItem("auto-sync-theme", String(value));
    } catch (error) {
      console.error("Failed to save auto theme sync setting:", error);
    }
    if (value) {
      applySystemTheme();
    }
  };

  /**
   * 切换深色/浅色模式
   * 切换时会禁用自动同步功能
   */
  const toggleDarkMode = () => {
    setAutoSync(false);
    setDarkMode(!isDark.value);
  };

  /**
   * 应用系统主题设置
   * 根据系统颜色方案偏好设置主题
   */
  const applySystemTheme = () => {
    try {
      const query = getMediaQuery();
      setDarkMode(query.matches);
    } catch (error) {
      console.error("Failed to read system theme:", error);
      // 默认使用浅色模式
      setDarkMode(false);
    }
  };

  const handleSystemThemeChange = (event: MediaQueryListEvent) => {
    if (autoSync.value) {
      setDarkMode(event.matches);
    }
  };

  const getMediaQuery = () => {
    if (!mediaQuery) {
      mediaQuery = window.matchMedia("(prefers-color-scheme: dark)");
    }
    return mediaQuery;
  };

  /**
   * 初始化主题设置
   * 从本地存储加载保存的主题设置
   */
  const init = () => {
    if (initialized) return;
    initialized = true;
    try {
      const savedTheme = localStorage.getItem("theme");
      const savedAutoSync = localStorage.getItem("auto-sync-theme");

      autoSync.value = savedAutoSync !== "false";

      if (autoSync.value) {
        applySystemTheme();
      } else {
        setDarkMode(savedTheme === "dark");
      }

      getMediaQuery().addEventListener("change", handleSystemThemeChange);
    } catch (error) {
      console.error("Failed to initialize theme setting:", error);
      // 使用默认设置
      setDarkMode(false);
    }
  };

  /**
   * 清理系统主题监听器
   * 在根组件卸载时调用，避免开发热更新或测试环境残留监听器
   */
  const cleanup = () => {
    mediaQuery?.removeEventListener("change", handleSystemThemeChange);
    initialized = false;
  };

  return { isDark, autoSync, setAutoSync, setDarkMode, toggleDarkMode, applySystemTheme, init, cleanup };
});
