import { ElMessage } from "element-plus";
import { ref } from "vue";
import { useI18n } from "vue-i18n";
import { getErrorMessage as resolveErrorMessage } from "@/utils/apiError";

/**
 * 统一的 API 错误处理 composable。
 *
 * 抽取自 Login/Register/Upload 等组件里重复的 `getAuthErrorMessage` /
 * `getUploadErrorMessage` 模式，避免每个组件各自手写一遍 axios 错误解包。
 *
 * - `getErrorMessage(error, fallback)` 返回可直接展示给用户的错误文案。
 * - `handleError(error, fallback)` 同时弹出一个 ElMessage.error。
 * - `message` ref 可用于在模板中绑定错误展示（例如 Admin 信息面板的 retry 状态）。
 */
export function useApiError() {
  const { t } = useI18n();
  const message = ref<string>("");

  const getErrorMessage = (error: unknown, fallback = t("common.operation_failed")): string => {
    return resolveErrorMessage(error, fallback);
  };

  const handleError = (error: unknown, fallback = t("common.operation_failed")): string => {
    const msg = getErrorMessage(error, fallback);
    message.value = msg;
    ElMessage.error(msg);
    return msg;
  };

  const clear = () => {
    message.value = "";
  };

  return { message, getErrorMessage, handleError, clear };
}
