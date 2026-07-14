import { describe, it, expect, beforeEach, vi } from "vitest"
import { setActivePinia, createPinia } from "pinia"
import { useThemeStore } from "../theme"

/** 在调用 store DOM 方法前 mock 全局 document / window */
function stubDOM() {
  const classList = new Set<string>()
  const listeners: Array<(e: { matches: boolean }) => void> = []

  vi.stubGlobal("document", {
    documentElement: {
      classList: {
        add: (c: string) => classList.add(c),
        remove: (c: string) => classList.delete(c),
        contains: (c: string) => classList.has(c),
      },
    },
  })

  vi.stubGlobal("window", {
    matchMedia: (_query: string) => ({
      matches: true, // system prefers dark
      addEventListener: (_event: string, fn: (e: { matches: boolean }) => void) => {
        listeners.push(fn)
      },
      removeEventListener: (_event: string, fn: (e: { matches: boolean }) => void) => {
        const i = listeners.indexOf(fn)
        if (i >= 0) listeners.splice(i, 1)
      },
    }),
  })

  return { classList, listeners }
}

describe("useThemeStore", () => {
  beforeEach(() => {
    localStorage.clear()
    setActivePinia(createPinia())
    vi.restoreAllMocks()
  })

  // ── 初始状态 ───────────────────────────────────────────

  it("starts with isDark = false and autoSync = true", () => {
    const store = useThemeStore()
    expect(store.isDark).toBe(false)
    expect(store.autoSync).toBe(true)
  })

  // ── setDarkMode ────────────────────────────────────────

  it("setDarkMode(true) persists dark to localStorage and adds class", () => {
    const { classList } = stubDOM()
    const store = useThemeStore()

    store.setDarkMode(true)
    expect(store.isDark).toBe(true)
    expect(localStorage.getItem("theme")).toBe("dark")
    expect(classList.has("dark")).toBe(true)
  })

  it("setDarkMode(false) persists light to localStorage and removes class", () => {
    const { classList } = stubDOM()
    const store = useThemeStore()

    store.setDarkMode(false)
    expect(store.isDark).toBe(false)
    expect(localStorage.getItem("theme")).toBe("light")
    expect(classList.has("dark")).toBe(false)
  })

  // ── toggleDarkMode ─────────────────────────────────────

  it("toggleDarkMode flips state and disables autoSync", () => {
    stubDOM()
    const store = useThemeStore()
    store.setAutoSync(true)
    // setAutoSync(true) calls applySystemTheme() which sets isDark based on mock (matches:true → dark)
    // So isDark is now true. Toggle to false.
    expect(store.isDark).toBe(true) // system prefers dark

    store.toggleDarkMode()
    expect(store.isDark).toBe(false)
    expect(store.autoSync).toBe(false)

    store.toggleDarkMode()
    expect(store.isDark).toBe(true)
  })

  // ── init ───────────────────────────────────────────────

  it("init reads saved dark theme from localStorage", () => {
    stubDOM()
    localStorage.setItem("theme", "dark")
    localStorage.setItem("auto-sync-theme", "false")

    const store = useThemeStore()
    store.init()

    expect(store.autoSync).toBe(false)
    expect(store.isDark).toBe(true)
  })

  it("init applies system theme when autoSync is enabled", () => {
    stubDOM()
    localStorage.setItem("auto-sync-theme", "true")

    const store = useThemeStore()
    store.init()

    expect(store.autoSync).toBe(true)
    expect(store.isDark).toBe(true) // system mock returns prefers-dark
  })

  it("init is idempotent", () => {
    stubDOM()
    const store = useThemeStore()
    store.init()
    store.setDarkMode(false) // user overrides
    store.init() // second call should not re-apply
    expect(store.isDark).toBe(false)
  })

  it("follows later system theme changes while auto sync is enabled", () => {
    const { classList, listeners } = stubDOM()
    const store = useThemeStore()
    store.init()

    expect(listeners).toHaveLength(1)
    listeners[0]?.({ matches: false })
    expect(store.isDark).toBe(false)
    expect(classList.has("dark")).toBe(false)

    store.setAutoSync(false)
    listeners[0]?.({ matches: true })
    expect(store.isDark).toBe(false)
  })

  // ── cleanup ────────────────────────────────────────────

  it("cleanup removes system theme listener", () => {
    const { listeners } = stubDOM()
    const store = useThemeStore()
    store.init()
    expect(listeners).toHaveLength(1)

    store.cleanup()
    expect(listeners).toHaveLength(0)
  })
})
