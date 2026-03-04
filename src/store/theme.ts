import { defineStore } from 'pinia'
import { ref, watch, onMounted } from 'vue'

export const useThemeStore = defineStore('theme', () => {
  const isDark = ref(false)
  const autoSync = ref(true)

  const setDarkMode = (value: boolean) => {
    isDark.value = value
    if (value) {
      document.documentElement.classList.add('dark')
      localStorage.setItem('theme', 'dark')
    } else {
      document.documentElement.classList.remove('dark')
      localStorage.setItem('theme', 'light')
    }
  }

  const setAutoSync = (value: boolean) => {
    autoSync.value = value
    localStorage.setItem('auto-sync-theme', String(value))
    if (value) {
      applySystemTheme()
    }
  }

  const toggleDarkMode = () => {
    setAutoSync(false)
    setDarkMode(!isDark.value)
  }

  const applySystemTheme = () => {
    const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
    setDarkMode(mediaQuery.matches)
  }

  onMounted(() => {
    const savedTheme = localStorage.getItem('theme')
    const savedAutoSync = localStorage.getItem('auto-sync-theme')

    autoSync.value = savedAutoSync !== 'false'

    if (autoSync.value) {
      applySystemTheme()
    } else {
      setDarkMode(savedTheme === 'dark')
    }
  })

  watch(autoSync, (newValue) => {
    if (newValue) {
      applySystemTheme()
    }
  })

  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', (e) => {
    if (autoSync.value) {
      setDarkMode(e.matches)
    }
  })

  return { isDark, autoSync, setAutoSync, toggleDarkMode }
})
