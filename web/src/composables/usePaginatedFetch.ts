import { ElMessage } from "element-plus";
import { computed, ref, type Ref } from "vue";
import { useI18n } from "vue-i18n";
import type { PaginatedData } from "@/types/api";
import { getErrorMessage } from "@/utils/apiError";
import request from "@/utils/request";

export interface UsePaginatedFetchOptions {
  /** 初始每页数量，默认 10 */
  initialPageSize?: number;
  /** 额外的查询参数（例如搜索关键词 q） */
  extraParams?: Ref<Record<string, unknown>>;
  /** 加载失败时的提示文案 i18n key，默认 common.load_failed */
  errorMessageKey?: string;
}

/**
 * 分页列表获取 composable。
 *
 * 抽取自 Home / MusicManagement / UserManagement 中重复的
 * `loading / items / total / currentPage / pageSize / fetchXxx` 状态机。
 *
 * 用法：
 * ```ts
 * const { items, total, loading, error, currentPage, pageSize, fetch } =
 *   usePaginatedFetch<Music>("/musics", { extraParams: computed(() => ({ q: query.value })) });
 * ```
 */
export function usePaginatedFetch<T>(url: string, options: UsePaginatedFetchOptions = {}) {
  const { t } = useI18n();
  const { initialPageSize = 10, extraParams } = options;

  const loading = ref(false);
  const error = ref<string | null>(null);
  const items = ref<T[]>([]) as Ref<T[]>;
  const total = ref(0);
  const currentPage = ref(1);
  const pageSize = ref(initialPageSize);

  const hasError = computed(() => error.value !== null);
  const isEmpty = computed(() => !loading.value && !error.value && items.value.length === 0);

  const buildParams = (): Record<string, unknown> => {
    const params: Record<string, unknown> = {
      page: currentPage.value,
      page_size: pageSize.value,
    };
    if (extraParams?.value) {
      Object.assign(params, extraParams.value);
    }
    return params;
  };

  /** 拉取当前页数据 */
  const fetch = async (overrideUrl?: string) => {
    loading.value = true;
    error.value = null;
    try {
      const targetUrl = overrideUrl || url;
      const res = await request.get<PaginatedData<T>>(targetUrl, { params: buildParams() });
      items.value = res.data.items || [];
      total.value = res.data.total ?? 0;
    } catch (e) {
      error.value = getErrorMessage(e, t(options.errorMessageKey ?? "common.load_failed"));
      ElMessage.error(error.value);
    } finally {
      loading.value = false;
    }
  };

  /** 切换到指定页并拉取 */
  const goToPage = (page: number) => {
    currentPage.value = page;
    return fetch();
  };

  /** 重置到第 1 页并拉取（搜索/筛选条件变化时使用） */
  const resetAndFetch = () => {
    currentPage.value = 1;
    return fetch();
  };

  return {
    loading,
    error,
    hasError,
    isEmpty,
    items,
    total,
    currentPage,
    pageSize,
    fetch,
    goToPage,
    resetAndFetch,
  };
}
