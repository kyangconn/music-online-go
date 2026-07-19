import { reactive, ref } from "vue";
import { ElMessage } from "element-plus";
import { useI18n } from "vue-i18n";
import type { MusicMetadataFields } from "@/types/api";
import {
  emptyMusicMetadataFields,
  loadCachedMeta,
  parseAudioFile,
  removeCachedMeta,
  saveCachedMeta,
} from "@/utils/upload";

/**
 * 音频元数据解析 composable。
 *
 * 抽取自 SingleMusicUpload / BatchMusicUpload 中重复的
 * 「解析音频标签 + localStorage 缓存 + touched 字段保护」逻辑。
 *
 * `form` 中的字段如果用户已经手动编辑过（touched），不会被元数据覆盖。
 */
export function useAudioMetadata() {
  const { t } = useI18n();
  const parsing = ref(false);

  /** 表单字段，由调用方绑定到 v-model */
  const form = reactive<MusicMetadataFields>(emptyMusicMetadataFields());

  /** 记录哪些字段已被用户手动编辑过，避免解析结果覆盖用户输入 */
  const touched = reactive(
    Object.fromEntries(Object.keys(emptyMusicMetadataFields()).map((key) => [key, false])) as Record<
      keyof MusicMetadataFields,
      boolean
    >,
  );

  /** 将解析的元数据填入表单（跳过已 touched 的字段） */
  const applyMeta = (meta: MusicMetadataFields) => {
    (Object.keys(meta) as (keyof MusicMetadataFields)[]).forEach((key) => {
      const value = meta[key];
      if (touched[key] || (Array.isArray(value) ? value.length === 0 : !value)) return;
      if (Array.isArray(value)) {
        Object.assign(form, { [key]: [...value] });
      } else {
        Object.assign(form, { [key]: value });
      }
    });
  };

  const resetTouched = () => {
    (Object.keys(touched) as (keyof MusicMetadataFields)[]).forEach((key) => {
      touched[key] = false;
    });
  };

  const resetForm = () => {
    Object.assign(form, emptyMusicMetadataFields());
    resetTouched();
  };

  /**
   * 解析单个音频文件并填充表单。
   * 优先使用 localStorage 缓存；失败时弹出提示。返回解析结果。
   */
  const parseAndFill = async (file: File): Promise<MusicMetadataFields | null> => {
    const cached = loadCachedMeta(file);
    if (cached) {
      applyMeta(cached);
      return cached;
    }
    try {
      const meta = await parseAudioFile(file);
      saveCachedMeta(file, meta);
      applyMeta(meta);
      return meta;
    } catch {
      ElMessage.warning(t("add.read_tags_failed"));
      return null;
    }
  };

  /** 仅解析返回元数据，不写入表单（批量场景按行解析） */
  const parseSingle = async (file: File): Promise<MusicMetadataFields> => {
    const cached = loadCachedMeta(file);
    if (cached) return cached;
    const meta = await parseAudioFile(file);
    saveCachedMeta(file, meta);
    return meta;
  };

  return {
    form,
    touched,
    parsing,
    applyMeta,
    resetTouched,
    resetForm,
    parseAndFill,
    parseSingle,
    /** 透出工具函数，便于上传成功后清理缓存 */
    clearCache: removeCachedMeta,
  };
}
