import { createI18n } from 'vue-i18n'
import zhCN from './locales/zh-CN.json'
import en from './locales/en.json'

// 获取浏览器语言
const getBrowserLang = () => {
  const savedLang = localStorage.getItem('user-language')
  if (savedLang) {
    return savedLang
  }
  const lang = navigator.language.toLowerCase()
  // 识别到 zh-cn 显示 zh-cn，else 都是英文
  if (lang.includes('zh-cn')) {
    return 'zh-CN'
  }
  return 'en'
}

const locale = getBrowserLang()

// 动态设置 html 的 lang 属性，防止浏览器弹出翻译提示
document.documentElement.lang = locale === 'zh-CN' ? 'zh-CN' : 'en'

const i18n = createI18n({
  legacy: false, // 使用 Composition API
  locale: locale,
  fallbackLocale: 'en',
  messages: {
    'zh-CN': zhCN,
    'en': en
  }
})

export default i18n
