<script setup lang="ts">
import { Headset, PictureFilled, QuestionFilled } from "@element-plus/icons-vue";
import { ElMessage, ElMessageBox } from "element-plus";
import type { UploadFile, UploadInstance } from "element-plus";
import { onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { useRouter } from "vue-router";
import FileCard from "@/components/upload/FileCard.vue";
import { useAudioMetadata } from "@/composables/useAudioMetadata";
import { useAudioPreprocessor } from "@/composables/useAudioPreprocessor";
import { useApiError } from "@/composables/useApiError";
import { useMusicDuplicates } from "@/composables/useMusicDuplicates";
import { useMusicUpload } from "@/composables/useMusicUpload";
import { useUploadDraft } from "@/composables/useUploadDraft";
import { useUploadPolicy } from "@/composables/useUploadPolicy";
import type { MusicDuplicateCheckData } from "@/types/api";
import { applyMetadataSuggestion, getUploadValidationMessage, validateUploadFile } from "@/utils/upload";

const router = useRouter();
const { t } = useI18n();
const { getErrorMessage } = useApiError();

const coverFile = ref<File | null>(null);
const audioFile = ref<File | null>(null);
const coverUploadRef = ref<UploadInstance>();
const audioUploadRef = ref<UploadInstance>();
const description = ref("");
const audioHash = ref("");

const { form, touched, applyMeta, resetForm, clearCache } = useAudioMetadata();
const { preprocess } = useAudioPreprocessor();
const { checking, checkDuplicate, enrichExactMatch } = useMusicDuplicates();
const { clearDraft } = useUploadDraft(form, touched, description);
const { loading, uploadPercent, uploadOne } = useMusicUpload();
const { policy, loadPolicy } = useUploadPolicy();

const acceptFile = (file: File | undefined, kind: "audio" | "cover") => {
  if (!file) return null;
  const validation = validateUploadFile(file, kind, policy.value);
  if (!validation.valid) {
    ElMessage.error(getUploadValidationMessage(file, kind, validation, t));
    return null;
  }
  return file;
};

onMounted(() => {
  void loadPolicy();
});

/** 封面文件选择回调 */
const handleCoverChange = (file: UploadFile) => {
  const raw = acceptFile(file?.raw || undefined, "cover");
  coverFile.value = raw;
  if (!raw) coverUploadRef.value?.clearFiles();
};

const handleCoverExceed = (files: UploadFile[]) => {
  coverUploadRef.value?.clearFiles();
  const raw = acceptFile(files?.[0]?.raw as File | undefined, "cover");
  if (raw) coverFile.value = raw;
};

/** 移除封面 */
const removeCover = () => {
  coverFile.value = null;
  coverUploadRef.value?.clearFiles();
};

const selectCover = () => {
  const input = coverUploadRef.value?.$el?.querySelector("input") as HTMLInputElement | null;
  input?.click();
};

/** 音频文件选择回调，自动解析标签 */
const handleAudioChange = async (file: UploadFile) => {
  audioFile.value = acceptFile(file?.raw || undefined, "audio");
  if (!audioFile.value) {
    audioUploadRef.value?.clearFiles();
    return;
  }
  await preprocessAudio(audioFile.value);
};

/** 音频文件超限处理 */
const handleAudioExceed = async (files: UploadFile[]) => {
  audioUploadRef.value?.clearFiles();
  const raw = acceptFile(files?.[0]?.raw as File | undefined, "audio");
  if (!raw) return;
  audioFile.value = raw;
  await preprocessAudio(raw);
};

/** 移除音频 */
const removeAudio = () => {
  audioFile.value = null;
  audioHash.value = "";
  audioUploadRef.value?.clearFiles();
};

const selectAudio = () => {
  const input = audioUploadRef.value?.$el?.querySelector("input") as HTMLInputElement | null;
  input?.click();
};

const preprocessAudio = async (file: File) => {
  audioHash.value = "";
  try {
    const result = await preprocess(file);
    audioHash.value = result.hash;
    applyMeta(result.metadata);
    return true;
  } catch {
    ElMessage.warning(t("add.read_tags_failed"));
    return false;
  }
};

const resetUploadForm = () => {
  if (audioFile.value) clearCache(audioFile.value);
  coverFile.value = null;
  audioFile.value = null;
  audioHash.value = "";
  coverUploadRef.value?.clearFiles();
  audioUploadRef.value?.clearFiles();
  description.value = "";
  resetForm();
  clearDraft();
};

/** 提交音乐创建并上传文件 */
const handleSubmit = async () => {
  const title = form.title.trim();
  const artist = form.artist.trim();
  if (!title || !artist) {
    ElMessage.error(t("add.title_artist_required"));
    return;
  }
  let duplicateResult: MusicDuplicateCheckData;
  try {
    duplicateResult = await checkDuplicate(form, audioHash.value);
    applyMetadataSuggestion(form, duplicateResult.suggested_metadata);
  } catch (error) {
    ElMessage.error(getErrorMessage(error, t("add.duplicate_check_failed")));
    return;
  }

  if (duplicateResult.exact_match) {
    try {
      const enriched = await enrichExactMatch(duplicateResult);
      ElMessage.success(enriched ? t("add.duplicate_enriched") : t("add.exact_duplicate_skipped"));
      const existingID = duplicateResult.exact_match.id;
      resetUploadForm();
      void router.push(`/music/${existingID}`);
    } catch (error) {
      ElMessage.error(getErrorMessage(error, t("add.duplicate_enrich_failed")));
    }
    return;
  }

  if (duplicateResult.metadata_matches.length > 0) {
    const confirmed = await ElMessageBox.confirm(
      t("add.possible_duplicate_confirm", { count: duplicateResult.metadata_matches.length }),
      t("add.possible_duplicate_title"),
      { type: "warning", confirmButtonText: t("add.upload_anyway"), cancelButtonText: t("common.cancel") },
    )
      .then(() => true)
      .catch(() => false);
    if (!confirmed) return;
  }

  const result = await uploadOne({
    title,
    artist,
    intro: description.value.trim(),
    metadata: form,
    audio: audioFile.value,
    cover: coverFile.value,
  });
  if (result.success) {
    ElMessage.success(t("add.upload_success"));
    resetUploadForm();
  }
};
</script>

<template>
  <div class="single-upload">
    <div class="file-cards">
      <FileCard :file="coverFile" :label="$t('add.cover')" :icon="PictureFilled" @select="selectCover" @remove="removeCover">
        <el-upload
          class="card-upload-input"
          ref="coverUploadRef"
          :show-file-list="false"
          :limit="1"
          :auto-upload="false"
          :on-change="handleCoverChange"
          :on-exceed="handleCoverExceed"
          accept="image/*"
        >
          <span class="upload-input-trigger" />
        </el-upload>
      </FileCard>

      <FileCard :file="audioFile" :label="$t('add.audio')" :icon="Headset" @select="selectAudio" @remove="removeAudio">
        <el-upload
          class="card-upload-input"
          ref="audioUploadRef"
          :show-file-list="false"
          :limit="1"
          :auto-upload="false"
          :on-change="handleAudioChange"
          :on-exceed="handleAudioExceed"
          accept="audio/*"
        >
          <span class="upload-input-trigger" />
        </el-upload>
      </FileCard>
    </div>

    <el-form class="upload-form" label-position="top" :model="form">
      <el-row :gutter="20">
        <el-col :span="12">
          <el-form-item :label="$t('add.music_title')">
            <el-input v-model="form.title" @input="touched.title = true" />
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item :label="$t('add.music_artist')">
            <el-input v-model="form.artist" @input="touched.artist = true" />
          </el-form-item>
        </el-col>
      </el-row>
      <el-row :gutter="20">
        <el-col :span="8">
          <el-form-item :label="$t('add.music_album')">
            <el-input v-model="form.album" @input="touched.album = true" />
          </el-form-item>
        </el-col>
        <el-col :span="4">
          <el-form-item :label="$t('add.music_year')">
            <el-input v-model="form.year" placeholder="e.g. 2024" @input="touched.year = true" />
          </el-form-item>
        </el-col>
        <el-col :span="4">
          <el-form-item>
            <template #label>
              {{ $t("add.music_track") }}
              <el-tooltip :content="$t('add.music_track_help')" placement="top">
                <el-icon class="help-icon" :size="14"><QuestionFilled /></el-icon>
              </el-tooltip>
            </template>
            <el-input v-model="form.track" placeholder="e.g. 1" @input="touched.track = true" />
          </el-form-item>
        </el-col>
        <el-col :span="8">
          <el-form-item :label="$t('add.music_genre')">
            <el-input v-model="form.genre" placeholder="e.g. Rock; Alternative" @input="touched.genre = true" />
          </el-form-item>
        </el-col>
      </el-row>
      <el-row :gutter="20">
        <el-col :span="6">
          <el-form-item :label="$t('add.music_duration')">
            <el-input v-model="form.duration" placeholder="e.g. 03:45" />
          </el-form-item>
        </el-col>
        <el-col :span="18">
          <el-form-item :label="$t('add.music_description')">
            <el-input type="textarea" v-model="description" :rows="2" />
          </el-form-item>
        </el-col>
      </el-row>

      <el-form-item>
        <el-button type="primary" :loading="loading || checking" size="large" @click="handleSubmit">
          {{ $t("common.upload") }}
        </el-button>
        <el-button size="large" @click="router.back()">{{ $t("common.cancel") }}</el-button>
      </el-form-item>

      <div v-if="uploadPercent > 0" class="upload-progress">
        <el-progress :percentage="uploadPercent" :stroke-width="12" text-inside />
      </div>
    </el-form>
  </div>
</template>

<style scoped lang="scss">
.file-cards {
  display: flex;
  gap: $spacing-lg;
  margin-bottom: $spacing-2xl;
}

.single-upload {
  overflow: visible;
  padding: 2px;
}

/* el-upload 作为覆盖在 FileCard 上的透明输入层 */
.card-upload-input {
  position: absolute;
  inset: 0;
  opacity: 0;
  cursor: pointer;
  z-index: 1;

  :deep(.el-upload) {
    width: 100%;
    height: 100%;
  }
}

.upload-input-trigger {
  display: block;
  width: 100%;
  height: 100%;
}

.upload-form {
  overflow: visible;
  padding-inline: 2px;

  :deep(.el-form-item) {
    overflow: visible;
  }
}

.upload-progress {
  margin-top: $spacing-sm;
}

.help-icon {
  color: var(--text-light);
  margin-left: $spacing-xs;
  cursor: help;
}

@include mobile {
  .file-cards {
    flex-direction: column;
  }
}
</style>
