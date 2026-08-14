import axios, { type AxiosError, type AxiosRequestConfig } from "axios";
import { ElMessage } from "element-plus";
import type { ApiResponse, RefreshData } from "@/types/api";
import i18n from "@/i18n";

/** 内部标记：重放请求/refresh 请求不再触发 401 自动刷新 */
declare module "axios" {
  export interface AxiosRequestConfig {
    skipAuthRefresh?: boolean;
  }
}

/**
 * 短期 access token 只保存在内存中（不落 localStorage），刷新页面后通过
 * /users/refresh 的 httpOnly cookie 静默恢复。user store 通过 setAccessToken
 * 与本模块同步，避免 store → request 的循环依赖。
 */
let accessToken = "";

export function setAccessToken(token: string) {
  accessToken = token;
}

export function clearAccessToken() {
  accessToken = "";
}

const service = axios.create({
  baseURL: "/api/v1",
  timeout: 5000,
});

service.interceptors.request.use(
  /** 在每个请求发出前自动附加 Authorization 请求头 */
  (config) => {
    if (accessToken) {
      config.headers["Authorization"] = "Bearer " + accessToken;
    }
    return config;
  },
  /** 处理请求发送阶段的错误 */
  (error) => Promise.reject(error),
);

/** 进行中的 refresh 请求（单飞）：多个 401 只触发一次刷新 */
let refreshPromise: Promise<boolean> | null = null;

/** 多标签页并发刷新时服务端返回的 401（宽限窗口内），重试一次即可成功 */
function isConcurrentRefreshError(error: AxiosError<ApiResponse>): boolean {
  return error.response?.status === 401 && error.response?.data?.error?.includes("concurrently") === true;
}

/** 触发一次 refresh，返回是否成功；并发调用共享同一个请求 */
function refreshAccessToken(): Promise<boolean> {
  if (!refreshPromise) {
    refreshPromise = (async () => {
      for (let attempt = 0; attempt < 2; attempt++) {
        try {
          const res = await service.post("/users/refresh", undefined, { skipAuthRefresh: true });
          const data = (res as unknown as ApiResponse<RefreshData>).data;
          accessToken = data.access_token;
          return true;
        } catch (error) {
          // 并发轮换（另一标签页刚刷新）：用新 cookie 重试；
          // 其他错误（会话撤销/过期）直接放弃。
          if (attempt === 0 && isConcurrentRefreshError(error as AxiosError<ApiResponse>)) {
            continue;
          }
          clearAccessToken();
          return false;
        }
      }
      clearAccessToken();
      return false;
    })().finally(() => {
      refreshPromise = null;
    });
  }
  return refreshPromise;
}

/** 登出后跳转登录页，保留当前路径作为 redirect */
function redirectToLogin() {
  const currentLocation = window.location.pathname + window.location.search + window.location.hash;
  window.location.href = `/login?redirect=${encodeURIComponent(currentLocation)}`;
}

service.interceptors.response.use(
  /** 解包 axios 响应，将原始 API 响应体直接传递给调用方 */
  (response) => {
    const res = response.data;
    return res;
  },
  /** 401 时单飞刷新并重放原请求；刷新失败则清空本地状态并跳转登录 */
  async (error: AxiosError<ApiResponse>) => {
    const config = error.config as (AxiosRequestConfig & { skipAuthRefresh?: boolean }) | undefined;
    const status = error.response?.status ?? 0;
    const isAuthPath = config?.url === "/users/login" || config?.url === "/users/refresh";

    if (status === 401 && config && !config.skipAuthRefresh && !isAuthPath) {
      const refreshed = await refreshAccessToken();
      if (refreshed) {
        // 用新 token 重放原请求（仅一次：重放请求也带 skipAuthRefresh）。
        config.headers = config.headers ?? {};
        config.headers["Authorization"] = "Bearer " + accessToken;
        config.skipAuthRefresh = true;
        return service(config);
      }
    }

    if (error.response) {
      const data = error.response.data;

      if (status === 401) {
        const currentPath = window.location.pathname;
        const isAuthPage = currentPath === "/login" || currentPath === "/register";
        clearAccessToken();

        if (!isAuthPage) {
          if (data?.error?.includes("expired") || data?.error?.includes("Session")) {
            ElMessage.warning(i18n.global.t("common.session_expired"));
          } else {
            ElMessage.warning(i18n.global.t("common.please_login_to_continue"));
          }
          redirectToLogin();
        }
      } else if (status === 403) {
        console.warn("Forbidden:", data);
      } else if (status >= 500) {
        console.error("Server error:", status, data);
      }
    } else if (error.request) {
      console.error("Network error");
    }

    return Promise.reject(error);
  },
);

/** 类型安全的 HTTP 请求工具 */
const request = {
  /** GET 请求 */
  get<T = unknown>(url: string, config?: AxiosRequestConfig): Promise<ApiResponse<T>> {
    return service.get(url, config) as Promise<ApiResponse<T>>;
  },
  /** POST 请求 */
  post<T = unknown>(url: string, data?: unknown, config?: AxiosRequestConfig): Promise<ApiResponse<T>> {
    return service.post(url, data, config) as Promise<ApiResponse<T>>;
  },
  /** PUT 请求 */
  put<T = unknown>(url: string, data?: unknown, config?: AxiosRequestConfig): Promise<ApiResponse<T>> {
    return service.put(url, data, config) as Promise<ApiResponse<T>>;
  },
  /** PATCH 请求 */
  patch<T = unknown>(url: string, data?: unknown, config?: AxiosRequestConfig): Promise<ApiResponse<T>> {
    return service.patch(url, data, config) as Promise<ApiResponse<T>>;
  },
  /** DELETE 请求 */
  delete<T = unknown>(url: string, config?: AxiosRequestConfig): Promise<ApiResponse<T>> {
    return service.delete(url, config) as Promise<ApiResponse<T>>;
  },
};

export default request;
