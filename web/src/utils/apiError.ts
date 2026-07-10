import axios from "axios";

/** 后端错误响应里可能出现的字段 */
interface ErrorBody {
  error?: string;
  message?: string;
}

/**
 * 从未知错误中提取可展示给用户的文案。
 *
 * 这是 `useApiError` composable 的纯函数版本，供非组件场景
 * （例如在 composable 内部、store action、工具函数）使用。
 *
 * 优先级：axios 响应体 `error` → 响应体 `message` → `error.message` → fallback。
 */
export function getErrorMessage(error: unknown, fallback = "Operation failed"): string {
  if (axios.isAxiosError<ErrorBody>(error)) {
    const data = error.response?.data;
    return data?.error || data?.message || error.message || fallback;
  }
  if (error instanceof Error && error.message) return error.message;
  return fallback;
}
