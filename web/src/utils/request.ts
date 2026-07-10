import axios, { type AxiosRequestConfig } from "axios";
import { ElMessage } from "element-plus";
import type { ApiResponse } from "@/types/api";
import i18n from "@/i18n";

const service = axios.create({
  baseURL: "/api/v1",
  timeout: 5000,
});

service.interceptors.request.use(
  /** 在每个请求发出前自动附加 Authorization 请求头 */
  (config) => {
    const token = localStorage.getItem("token");
    if (token) {
      config.headers["Authorization"] = "Bearer " + token;
    }
    return config;
  },
  /** 处理请求发送阶段的错误 */
  (error) => Promise.reject(error),
);

service.interceptors.response.use(
  /** 解包 axios 响应，将原始 API 响应体直接传递给调用方 */
  (response) => {
    const res = response.data;
    return res;
  },
  /** 统一处理 HTTP 错误响应：401 自动登出、403 记录警告、5xx 记录错误 */
  (error) => {
    if (error.response) {
      const status = error.response.status;
      const data = error.response.data;

      if (status === 401) {
        const currentPath = window.location.pathname;
        const isAuthPage = currentPath === "/login" || currentPath === "/register";
        localStorage.removeItem("token");
        localStorage.removeItem("user");

        if (!isAuthPage) {
          if (data?.error?.includes("expired")) {
            ElMessage.warning(i18n.global.t("common.session_expired"));
          } else {
            ElMessage.warning(i18n.global.t("common.please_login_to_continue"));
          }
        }

        if (!isAuthPage) {
          const currentLocation = window.location.pathname + window.location.search + window.location.hash;
          window.location.href = `/login?redirect=${encodeURIComponent(currentLocation)}`;
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
  /** DELETE 请求 */
  delete<T = unknown>(url: string, config?: AxiosRequestConfig): Promise<ApiResponse<T>> {
    return service.delete(url, config) as Promise<ApiResponse<T>>;
  },
};

export default request;
