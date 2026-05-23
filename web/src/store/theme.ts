import { defineStore } from "pinia"
import { ref, watch, onMounted } from "vue"

/**
 * 主题状态管理存储
 * 管理应用的主题模式（深色/浅色）和系统主题自动同步功能
 */
export const useThemeStore = defineStore("theme", () => {
  /** 当前是否为深色模式 */
  const isDark = ref(false)
  /** 是否自动同步系统主题 */
  const autoSync = ref(true)

  /**
   * 设置深色模式
   * @param value - true 为深色模式，false 为浅色模式
   */
  const setDarkMode = (value: boolean) => {
    isDark.value = value
    try {
      if (value) {
        document.documentElement.classList.add("dark")
        localStorage.setItem("theme", "dark")
      } else {
        document.documentElement.classList.remove("dark")
        localStorage.setItem("theme", "light")
      }
    } catch (error) {
      console.error("设置主题失败:", error)
      // 即使存储失败，也应用主题到DOM，但可能无法持久化
    }
  }

  /**
   * 设置自动同步系统主题功能
   * @param value - true 启用自动同步，false 禁用自动同步
   */
  const setAutoSync = (value: boolean) => {
    autoSync.value = value
    try {
      localStorage.setItem("auto-sync-theme", String(value))
    } catch (error) {
      console.error("保存自动同步设置失败:", error)
    }
    if (value) {
      applySystemTheme()
    }
  }

  /**
   * 切换深色/浅色模式
   * 切换时会禁用自动同步功能
   */
  const toggleDarkMode = () => {
    setAutoSync(false)
    setDarkMode(!isDark.value)
  }

  /**
   * 应用系统主题设置
   * 根据系统颜色方案偏好设置主题
   */
  const applySystemTheme = () => {
    try {
      const mediaQuery = window.matchMedia("(prefers-color-scheme: dark)")
      setDarkMode(mediaQuery.matches)
    } catch (error) {
      console.error("获取系统主题失败:", error)
      // 默认使用浅色模式
      setDarkMode(false)
    }
  }

  /**
   * 初始化主题设置
   * 从本地存储加载保存的主题设置
   */
  onMounted(() => {
    try {
      const savedTheme = localStorage.getItem("theme")
      const savedAutoSync = localStorage.getItem("auto-sync-theme")

      autoSync.value = savedAutoSync !== "false"

      if (autoSync.value) {
        applySystemTheme()
      } else {
        setDarkMode(savedTheme === "dark")
      }
    } catch (error) {
      console.error("初始化主题设置失败:", error)
      // 使用默认设置
      setDarkMode(false)
    }
  })

  /**
   * 监听自动同步设置变化
   * 当启用自动同步时，立即应用系统主题
   */
  watch(autoSync, (newValue) => {
    if (newValue) {
      applySystemTheme()
    }
  })

  /**
   * 监听系统主题变化
   * 当系统主题变化且启用了自动同步时，更新应用主题
   */
  try {
    window.matchMedia("(prefers-color-scheme: dark)").addEventListener("change", (e) => {
      if (autoSync.value) {
        setDarkMode(e.matches)
      }
    })
  } catch (error) {
    console.error("监听系统主题变化失败:", error)
  }

  return { isDark, autoSync, setAutoSync, setDarkMode, toggleDarkMode, applySystemTheme }
})
