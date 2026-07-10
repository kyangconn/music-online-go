import { reactive, ref } from "vue";
import { ElMessage } from "element-plus";
import { useI18n } from "vue-i18n";
import type { MusicMetadataFields } from "@/types/api";
import { loadCachedMeta, parseAudioFile, removeCachedMeta, saveCachedMeta } from "@/utils/upload";

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
  const form = reactive<MusicMetadataFields>({
    title: "",
    artist: "",
    album: "",
    year: "",
    track: "",
    genre: "",
    duration: "",
  });

  /** 记录哪些字段已被用户手动编辑过，避免解析结果覆盖用户输入 */
  const touched = reactive<Record<keyof MusicMetadataFields, boolean>>({
    title: false,
    artist: false,
    album: false,
    year: false,
    track: false,
    genre: false,
    duration: false,
  });

  /** 将解析的元数据填入表单（跳过已 touched 的字段） */
  const applyMeta = (meta: MusicMetadataFields) => {
    (Object.keys(meta) as (keyof MusicMetadataFields)[]).forEach((key) => {
      const value = meta[key];
      if (key === "duration") {
        if (value) form[key] = value;
        return;
      }
      if (!touched[key] && value) form[key] = value;
    });
  };

  const resetTouched = () => {
    (Object.keys(touched) as (keyof MusicMetadataFields)[]).forEach((key) => {
      touched[key] = false;
    });
  };

  const resetForm = () => {
    (Object.keys(form) as (keyof MusicMetadataFields)[]).forEach((key) => {
      form[key] = "";
    });
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
