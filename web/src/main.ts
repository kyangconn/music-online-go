/**
 * 应用主入口文件
 * 初始化Vue应用并配置所有必要的插件和全局设置
 */

import { Buffer } from "buffer";
import ElementPlus from "element-plus";
import "element-plus/theme-chalk/dark/css-vars.css";
import "./styles/element-plus.scss";
import "./styles/global.scss";
import "./styles/common.scss";
import { createPinia } from "pinia";
import { createApp } from "vue";
import App from "./App.vue";
import i18n from "./i18n";
import router from "./router";

// 创建Vue应用实例
const app = createApp(App);

// 安装插件
app.use(createPinia()); // 状态管理
app.use(router); // 路由
app.use(i18n); // 国际化
app.use(ElementPlus); // UI组件库

// 挂载应用到DOM
app.mount("#app");

// 全局设置Buffer对象（某些库可能需要）
window.Buffer = Buffer;

if ("serviceWorker" in navigator && import.meta.env.PROD) {
  window.addEventListener("load", () => {
    void navigator.serviceWorker.register("/sw.js").catch(() => undefined);
  });
}
