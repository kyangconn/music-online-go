import { type AxiosProgressEvent } from "axios";
import { ElMessage } from "element-plus";
import { ref } from "vue";
import { useI18n } from "vue-i18n";
import type { CreateMusicData, CreateMusicRequest, MusicMetadataFields } from "@/types/api";
import { useApiError } from "@/composables/useApiError";
import { useUploadPolicy } from "@/composables/useUploadPolicy";
import request from "@/utils/request";
import { getUploadValidationMessage, metadataToData, validateUploadFile } from "@/utils/upload";

export interface UploadSingleOptions {
  title: string;
  artist: string;
  intro?: string;
  metadata?: MusicMetadataFields;
  audio?: File | null;
  cover?: File | null;
  existingMusicId?: number;
  silent?: boolean;
}

export interface UploadSingleResult {
  /** 是否成功创建并（如有）上传文件 */
  success: boolean;
  /** 创建出的音乐 ID（创建成功时才有值） */
  musicId?: number;
  /** 文件是否已上传 */
  uploaded: boolean;
  /** 失败原因，批量上传结果页直接展示 */
  errorMessage?: string;
}

/**
 * 单曲上传流程 composable。
 *
 * 抽取自 SingleMusicUpload / BatchMusicUpload 中重复的
 * 「创建音乐记录 → 上传音频/封面」流程，统一进度、错误处理与文案。
 */
export function useMusicUpload() {
  const { t } = useI18n();
  const { getErrorMessage } = useApiError();
  const { policy, loadPolicy } = useUploadPolicy();
  const loading = ref(false);
  const uploadPercent = ref(0);
  let resetProgressTimer: ReturnType<typeof setTimeout> | undefined;

  /**
   * 创建一条音乐记录并（如提供文件）上传音频/封面。
   * 失败时弹出提示并返回 success=false（不抛异常）。
   */
  const uploadOne = async (options: UploadSingleOptions): Promise<UploadSingleResult> => {
    const {
      title,
      artist,
      intro = "",
      metadata,
      audio = null,
      cover = null,
      existingMusicId,
      silent = false,
    } = options;
    if (resetProgressTimer) clearTimeout(resetProgressTimer);
    await loadPolicy();
    if (audio) {
      const audioValidation = validateUploadFile(audio, "audio", policy.value);
      if (!audioValidation.valid) {
        const errorMessage = getUploadValidationMessage(audio, "audio", audioValidation, t);
        if (!silent) ElMessage.error(errorMessage);
        return { success: false, uploaded: false, musicId: existingMusicId, errorMessage };
      }
    }
    if (cover) {
      const coverValidation = validateUploadFile(cover, "cover", policy.value);
      if (!coverValidation.valid) {
        const errorMessage = getUploadValidationMessage(cover, "cover", coverValidation, t);
        if (!silent) ElMessage.error(errorMessage);
        return { success: false, uploaded: false, musicId: existingMusicId, errorMessage };
      }
    }

    loading.value = true;
    uploadPercent.value = audio || cover ? 1 : 0;
    let musicId = existingMusicId;
    try {
      if (!musicId) {
        const parsedMetadata = metadata
          ? metadataToData(metadata, title, artist)
          : { title, artist, album: "", year: 0, track_number: 0, genre: "", duration: 0 };
        const payload: CreateMusicRequest = {
          ...parsedMetadata,
          title,
          artist,
          intro,
          type: "single",
        };
        const res = await request.post<CreateMusicData>("/musics", payload);
        musicId = res.data?.id;
      }
      if (!musicId) {
        const errorMessage = t("add.create_record_failed");
        if (!silent) ElMessage.error(errorMessage);
        return { success: false, uploaded: false, errorMessage };
      }

      let uploaded = false;
      if (audio || cover) {
        const fd = new FormData();
        if (audio) fd.append("file", audio);
        if (cover) fd.append("cover", cover);
        await request.post(`/musics/${musicId}/upload`, fd, {
          headers: { "Content-Type": "multipart/form-data" },
          onUploadProgress: (event: AxiosProgressEvent) => {
            if (!event.total) return;
            uploadPercent.value = Math.max(1, Math.round((event.loaded / event.total) * 100));
          },
        });
        uploadPercent.value = 100;
        uploaded = true;
      }

      return { success: true, musicId, uploaded };
    } catch (error) {
      const errorMessage = getErrorMessage(error, t("add.upload_failed"));
      if (!silent) ElMessage.error(errorMessage);
      return { success: false, uploaded: false, musicId, errorMessage };
    } finally {
      loading.value = false;
      resetProgressTimer = setTimeout(() => {
        uploadPercent.value = 0;
      }, 800);
    }
  };

  return { loading, uploadPercent, uploadOne };
}
