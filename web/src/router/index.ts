/**
 * 应用路由配置
 * 定义所有页面路由、权限控制和路由守卫
 */

import { createRouter, createWebHistory } from "vue-router";
import BaseLayout from "@/layout/BaseLayout.vue";
import { useUserStore } from "@/store/user";
import Home from "@/views/Home.vue";

/**
 * 路由配置数组
 * 定义应用的所有路由路径、组件和元数据
 */
const routes = [
  {
    path: "/",
    component: BaseLayout,
    children: [
      {
        path: "",
        name: "Home",
        component: Home,
      },
      {
        path: "/login",
        name: "Login",
        component: () => import("@/views/auth/Login.vue").catch(() => Home),
      },
      {
        path: "/register",
        name: "Register",
        component: () => import("@/views/auth/Register.vue").catch(() => Home),
      },
      {
        path: "/profile",
        name: "Profile",
        component: () => import("@/views/user/Profile.vue").catch(() => Home),
      },
      {
        path: "/settings",
        name: "Settings",
        component: () => import("@/views/user/Settings.vue").catch(() => Home),
      },
      {
        path: "/music/:id",
        name: "MusicDetail",
        component: () => import("@/views/music/Detail.vue").catch(() => Home),
      },
      {
        path: "/music/add",
        name: "MusicAdd",
        component: () => import("@/views/music/Add.vue").catch(() => Home),
        meta: { requiresAuth: true },
      },
      {
        path: "/admin",
        name: "Admin",
        component: () => import("@/views/admin/Dashboard.vue").catch(() => Home),
        meta: { requiresAdmin: true },
      },
    ],
  },
];

/**
 * 创建路由实例
 * 使用Web History模式，支持干净的URL
 */
const router = createRouter({
  history: createWebHistory(),
  routes,
});

/**
 * 全局路由守卫
 * 在每次路由跳转前执行，用于权限控制和访问限制
 * @param to - 目标路由
 * @param _from - 来源路由
 * @param next - 路由跳转函数
 */
router.beforeEach((to, _from, next) => {
  const userStore = useUserStore();

  // 检查是否需要认证
  if (to.meta.requiresAuth && !userStore.isLoggedIn) {
    next("/login");
  }
  // 检查是否需要管理员权限
  else if (to.meta.requiresAdmin && !userStore.isAdmin) {
    next("/");
  }
  // 允许访问
  else {
    next();
  }
});

export default router;
