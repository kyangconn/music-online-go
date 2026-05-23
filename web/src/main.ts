/**
 * 应用主入口文件
 * 初始化Vue应用并配置所有必要的插件和全局设置
 */

import * as ElementPlusIconsVue from "@element-plus/icons-vue"
import { Buffer } from "buffer"
import ElementPlus from "element-plus"
import "element-plus/dist/index.css"
import "element-plus/theme-chalk/dark/css-vars.css"
import "./style.css"
import "./styles/common.css"
import { createPinia } from "pinia"
import { createApp } from "vue"
import App from "./App.vue"
import i18n from "./i18n"
import router from "./router"

// 创建Vue应用实例
const app = createApp(App)

// 全局注册Element Plus图标组件
for (const [key, component] of Object.entries(ElementPlusIconsVue)) {
  app.component(key, component)
}

// 安装插件
app.use(createPinia()) // 状态管理
app.use(router) // 路由
app.use(i18n) // 国际化
app.use(ElementPlus) // UI组件库

// 挂载应用到DOM
app.mount("#app")

// 全局设置Buffer对象（某些库可能需要）
window.Buffer = Buffer
