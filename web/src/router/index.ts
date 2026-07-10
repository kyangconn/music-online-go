/**
 * 应用路由配置
 * 定义所有页面路由、权限控制和路由守卫
 */

import { ElMessage } from "element-plus";
import { nextTick } from "vue";
import { createRouter, createWebHistory } from "vue-router";
import i18n from "@/i18n";
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
        component: () => import("@/views/auth/Login.vue"),
      },
      {
        path: "/register",
        name: "Register",
        component: () => import("@/views/auth/Register.vue"),
      },
      {
        path: "/profile",
        name: "Profile",
        component: () => import("@/views/user/Profile.vue"),
        meta: { requiresAuth: true },
      },
      {
        path: "/settings",
        name: "Settings",
        component: () => import("@/views/user/Settings.vue"),
        meta: { requiresAuth: true },
      },
      {
        path: "/music/:id",
        name: "MusicDetail",
        component: () => import("@/views/music/Detail.vue"),
      },
      {
        path: "/music/:id/edit",
        name: "MusicEdit",
        component: () => import("@/views/music/Edit.vue"),
        meta: { requiresAuth: true },
      },
      {
        path: "/music/add",
        name: "MusicAdd",
        component: () => import("@/views/music/Add.vue"),
        meta: { requiresAuth: true },
      },
      {
        path: "/admin",
        name: "Admin",
        component: () => import("@/views/admin/Dashboard.vue"),
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
  scrollBehavior(_to, _from, savedPosition) {
    return savedPosition ?? { top: 0 };
  },
});

let finishViewTransition: (() => void) | null = null;
let activeViewTransition: ViewTransition | null = null;

router.beforeResolve((to, from) => {
  const startViewTransition = document.startViewTransition?.bind(document);
  const prefersReducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  if (
    !startViewTransition ||
    prefersReducedMotion ||
    from.matched.length === 0 ||
    to.fullPath === from.fullPath ||
    activeViewTransition
  ) {
    return true;
  }

  return new Promise<boolean>((resolveNavigation) => {
    const transition = startViewTransition(
      () =>
        new Promise<void>((resolveTransition) => {
          finishViewTransition = resolveTransition;
          resolveNavigation(true);
        }),
    );
    activeViewTransition = transition;
    const clearTransition = () => {
      if (activeViewTransition === transition) activeViewTransition = null;
    };
    void transition.finished.then(clearTransition, clearTransition);
  });
});

router.afterEach(async () => {
  const finish = finishViewTransition;
  if (!finish) return;
  await nextTick();
  if (finishViewTransition === finish) finishViewTransition = null;
  finish();
});

router.onError((error) => {
  finishViewTransition?.();
  finishViewTransition = null;
  console.error("Route navigation failed", error);
  ElMessage.error(i18n.global.t("common.page_load_failed"));
});

/**
 * 全局路由守卫
 * 在每次路由跳转前执行，用于权限控制和访问限制
 * @param to - 目标路由
 */
router.beforeEach((to) => {
  const userStore = useUserStore();
  const requiresAuth = Boolean(to.meta.requiresAuth || to.meta.requiresAdmin);
  const redirect = { path: "/login", query: { redirect: to.fullPath } };

  // 检查是否需要认证
  if (requiresAuth && (!userStore.isLoggedIn || isTokenExpired(userStore.token))) {
    userStore.logout();
    return redirect;
  }
  // 检查是否需要管理员权限
  if (to.meta.requiresAdmin && !userStore.isAdmin) {
    return "/";
  }
  return true;
});

const isTokenExpired = (token: string) => {
  if (!token) return true;
  try {
    const payload = token.split(".")[1];
    if (!payload) return true;
    const normalized = payload.replace(/-/g, "+").replace(/_/g, "/");
    const decoded = JSON.parse(atob(normalized)) as { exp?: number };
    return typeof decoded.exp !== "number" || decoded.exp * 1000 <= Date.now();
  } catch (_e) {
    return true;
  }
};

export default router;
