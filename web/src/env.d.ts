/// <reference types="vite/client" />

import type { Buffer } from "buffer";

/**
 * Vue 单文件组件模块声明
 * 让 TypeScript 识别 .vue 文件的导入
 */
declare module "*.vue" {
  import type { DefineComponent } from "vue";
  const component: DefineComponent<Record<string, unknown>, Record<string, unknown>, unknown>;
  export default component;
}

declare global {
  interface Window {
    Buffer: typeof Buffer;
  }
}
