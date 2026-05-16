import { createI18n } from 'vue-i18n'

const localeModules = import.meta.glob<{ default: Record<string, string> }>(
  './*/*.json',
  { eager: true },
)

function buildMessages(): Record<string, Record<string, Record<string, string>>> {
  const messages: Record<string, Record<string, Record<string, string>>> = {}

  for (const [path, mod] of Object.entries(localeModules)) {
    const parts = path.split('/')
    const locale = parts[1]
    const fileName = parts[2]
    if (!locale || !fileName) continue

    const namespace = fileName.replace('.json', '')

    if (!messages[locale]) messages[locale] = {}
    messages[locale][namespace] = mod.default
  }

  return messages
}

const getBrowserLang = () => {
  const savedLang = localStorage.getItem('user-language')
  if (savedLang) return savedLang
  const lang = navigator.language.toLowerCase()
  if (lang.includes('zh-cn')) return 'zh-CN'
  return 'en-US'
}

const locale = getBrowserLang()

document.documentElement.lang = locale === 'zh-CN' ? 'zh-CN' : 'en-US'

const i18n = createI18n({
  legacy: false,
  locale,
  fallbackLocale: 'en-US',
  messages: buildMessages(),
})

export default i18n
