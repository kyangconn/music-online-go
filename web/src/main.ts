/**
 * 应用主入口文件
 * 初始化Vue应用并配置所有必要的插件和全局设置
 */

import { Buffer } from "buffer";
import {
  ElButton,
  ElCard,
  ElCheckbox,
  ElCol,
  ElContainer,
  ElDialog,
  ElDivider,
  ElDropdown,
  ElDropdownItem,
  ElDropdownMenu,
  ElEmpty,
  ElFooter,
  ElForm,
  ElFormItem,
  ElHeader,
  ElIcon,
  ElImage,
  ElInput,
  ElInputNumber,
  ElLoading,
  ElMain,
  ElOption,
  ElPagination,
  ElProgress,
  ElRadioButton,
  ElRadioGroup,
  ElResult,
  ElRow,
  ElSelect,
  ElSkeleton,
  ElSlider,
  ElSwitch,
  ElTabPane,
  ElTable,
  ElTableColumn,
  ElTabs,
  ElTag,
  ElTooltip,
  ElUpload,
} from "element-plus";
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

// 按需注册 Element Plus 组件（避免引入全部 ~300 个图标）
const epComponents = [
  ElButton,
  ElCard,
  ElCheckbox,
  ElCol,
  ElContainer,
  ElDialog,
  ElDivider,
  ElDropdown,
  ElDropdownItem,
  ElDropdownMenu,
  ElEmpty,
  ElFooter,
  ElForm,
  ElFormItem,
  ElHeader,
  ElIcon,
  ElImage,
  ElInput,
  ElInputNumber,
  ElMain,
  ElOption,
  ElPagination,
  ElProgress,
  ElRadioButton,
  ElRadioGroup,
  ElResult,
  ElRow,
  ElSelect,
  ElSkeleton,
  ElSlider,
  ElSwitch,
  ElTabPane,
  ElTable,
  ElTableColumn,
  ElTabs,
  ElTag,
  ElTooltip,
  ElUpload,
];
epComponents.forEach((comp) => app.component(comp.name!, comp));

// 注册 v-loading 指令
app.directive("loading", ElLoading.directive);

// 安装插件
app.use(createPinia()); // 状态管理
app.use(router); // 路由
app.use(i18n); // 国际化

// 挂载应用到DOM
app.mount("#app");

// 全局设置Buffer对象（某些库可能需要）
window.Buffer = Buffer;

if ("serviceWorker" in navigator && import.meta.env.PROD) {
  window.addEventListener("load", () => {
    void navigator.serviceWorker.register("/sw.js").catch(() => undefined);
  });
}
