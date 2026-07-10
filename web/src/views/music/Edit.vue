<script setup lang="ts">
import axios, { type AxiosProgressEvent } from "axios";
import { Delete, Headset, PictureFilled, UploadFilled } from "@element-plus/icons-vue";
import type { FormInstance, FormRules } from "element-plus";
import { ElMessage, ElMessageBox } from "element-plus";
import { computed, onMounted, onUnmounted, reactive, ref } from "vue";
import { useI18n } from "vue-i18n";
import { useRoute, useRouter } from "vue-router";
import type { Music, UpdateMusicRequest } from "@/types/api";
import MusicCover from "@/components/music/MusicCover.vue";
import { useUploadPolicy } from "@/composables/useUploadPolicy";
import { useUserStore } from "@/store/user";
import request from "@/utils/request";
import {
  formatFileSize,
  formatDuration,
  getUploadValidationMessage,
  parseDurationSeconds,
  validateUploadFile,
} from "@/utils/upload";

const route = useRoute();
const router = useRouter();
const userStore = useUserStore();
const { t } = useI18n();
const { policy, loadPolicy } = useUploadPolicy();
const id = route.params.id as string;

const formRef = ref<FormInstance>();
const coverInputRef = ref<HTMLInputElement>();
const audioInputRef = ref<HTMLInputElement>();
const loading = ref(true);
const saving = ref(false);
const deleting = ref(false);
const uploadPercent = ref(0);
const music = ref<Music | null>(null);
const coverFile = ref<File | null>(null);
const audioFile = ref<File | null>(null);
const coverPreviewUrl = ref("");

interface EditMusicForm {
  title: string;
  artist: string;
  album: string;
  year?: number;
  trackNumber?: number;
  genre: string;
  duration: string;
  intro: string;
}

const form = reactive<EditMusicForm>({
  title: "",
  artist: "",
  album: "",
  year: undefined,
  trackNumber: undefined,
  genre: "",
  duration: "",
  intro: "",
});

const rules = reactive<FormRules>({
  title: [{ required: true, message: t("music.title_artist_required"), trigger: "blur" }],
  artist: [{ required: true, message: t("music.title_artist_required"), trigger: "blur" }],
});

interface ApiErrorResponse {
  error?: string;
  message?: string;
}

const getErrorMessage = (error: unknown, fallback: string) => {
  if (axios.isAxiosError<ApiErrorResponse>(error)) {
    const data = error.response?.data;
    return data?.error || data?.message || error.message || fallback;
  }
  if (error instanceof Error && error.message) return error.message;
  return fallback;
};

const canManage = computed(() => {
  if (!music.value || !userStore.user) return false;
  return userStore.isAdmin || music.value.user_id === userStore.user.id;
});

const coverSrc = computed(() => coverPreviewUrl.value || music.value?.img || music.value?.cover_url || "");

const revokeCoverPreview = () => {
  if (coverPreviewUrl.value) {
    URL.revokeObjectURL(coverPreviewUrl.value);
    coverPreviewUrl.value = "";
  }
};

const fetchMusic = async () => {
  loading.value = true;
  try {
    const res = await request.get<Music>(`/musics/${id}`);
    music.value = res.data;
    form.title = res.data.title;
    form.artist = res.data.artist;
    form.album = res.data.album || "";
    form.year = res.data.year || undefined;
    form.trackNumber = res.data.track_number || undefined;
    form.genre = res.data.genre || "";
    form.duration = res.data.duration ? formatDuration(res.data.duration) : "";
    form.intro = res.data.intro || "";

    if (!canManage.value) {
      ElMessage.warning(t("music.permission_denied"));
      router.replace(`/music/${id}`);
    }
  } catch (error) {
    ElMessage.error(getErrorMessage(error, t("music.load_failed")));
    router.replace("/");
  } finally {
    loading.value = false;
  }
};

const selectCover = () => {
  coverInputRef.value?.click();
};

const selectAudio = () => {
  audioInputRef.value?.click();
};

const acceptFile = (file: File | null, kind: "audio" | "cover") => {
  if (!file) return null;
  const validation = validateUploadFile(file, kind, policy.value);
  if (!validation.valid) {
    ElMessage.error(getUploadValidationMessage(file, kind, validation, t));
    return null;
  }
  return file;
};

const handleCoverSelected = (event: Event) => {
  const input = event.target as HTMLInputElement;
  const file = acceptFile(input.files?.[0] || null, "cover");
  coverFile.value = file;
  revokeCoverPreview();
  if (file) {
    coverPreviewUrl.value = URL.createObjectURL(file);
  } else {
    input.value = "";
  }
};

const handleAudioSelected = (event: Event) => {
  const input = event.target as HTMLInputElement;
  const file = acceptFile(input.files?.[0] || null, "audio");
  audioFile.value = file;
  if (!file) input.value = "";
};

const uploadSelectedFiles = async () => {
  if (!audioFile.value && !coverFile.value) return;

  const formData = new FormData();
  if (audioFile.value) formData.append("file", audioFile.value);
  if (coverFile.value) formData.append("cover", coverFile.value);

  uploadPercent.value = 1;
  await request.post<Music>(`/musics/${id}/upload`, formData, {
    headers: { "Content-Type": "multipart/form-data" },
    onUploadProgress: (event: AxiosProgressEvent) => {
      if (!event.total) return;
      uploadPercent.value = Math.max(1, Math.round((event.loaded / event.total) * 100));
    },
  });
  uploadPercent.value = 100;
};

const handleSave = async (formEl: FormInstance | undefined) => {
  if (!formEl || !canManage.value) return;
  await formEl.validate(async (valid) => {
    if (!valid) return;

    saving.value = true;
    try {
      const payload: UpdateMusicRequest = {
        title: form.title.trim(),
        artist: form.artist.trim(),
        album: form.album.trim(),
        year: form.year,
        track_number: form.trackNumber,
        genre: form.genre.trim(),
        duration: parseDurationSeconds(form.duration),
        intro: form.intro.trim(),
        type: music.value?.type || "single",
      };
      const updateRes = await request.put<Music>(`/musics/${id}`, payload);
      music.value = updateRes.data;

      await uploadSelectedFiles();
      const refreshed = await request.get<Music>(`/musics/${id}`);
      music.value = refreshed.data;

      coverFile.value = null;
      audioFile.value = null;
      revokeCoverPreview();
      if (coverInputRef.value) coverInputRef.value.value = "";
      if (audioInputRef.value) audioInputRef.value.value = "";
      ElMessage.success(t("music.save_success"));
      router.push(`/music/${id}`);
    } catch (error) {
      ElMessage.error(getErrorMessage(error, t("music.save_failed")));
    } finally {
      saving.value = false;
      setTimeout(() => {
        uploadPercent.value = 0;
      }, 800);
    }
  });
};

const handleDelete = async () => {
  if (!music.value || !canManage.value) return;

  try {
    await ElMessageBox.confirm(t("music.delete_confirm", { title: music.value.title }), t("music.delete_music"), {
      confirmButtonText: t("common.delete"),
      cancelButtonText: t("common.cancel"),
      type: "warning",
    });
    deleting.value = true;
    await request.delete(`/musics/${id}`);
    ElMessage.success(t("music.delete_success"));
    router.push("/profile");
  } catch (error) {
    if (error !== "cancel" && error !== "close") {
      ElMessage.error(getErrorMessage(error, t("music.delete_failed")));
    }
  } finally {
    deleting.value = false;
  }
};

onMounted(() => {
  void loadPolicy();
  void fetchMusic();
});
onUnmounted(revokeCoverPreview);
</script>

<template>
  <div class="page-section edit-container">
    <el-card class="content-card" shadow="never">
      <div class="edit-header">
        <div>
          <h2>{{ $t("music.edit_title") }}</h2>
          <p>{{ $t("music.edit_subtitle") }}</p>
        </div>
        <el-button @click="router.back()">{{ $t("common.back") }}</el-button>
      </div>

      <el-skeleton v-if="loading" :rows="6" animated />
      <div v-else-if="music" class="edit-body">
        <div class="media-panel">
          <div class="cover-preview">
            <MusicCover :src="coverSrc" />
          </div>
          <div class="file-actions">
            <span class="file-caption">{{ $t("music.current_cover") }}</span>
            <el-button :icon="PictureFilled" @click="selectCover">{{ $t("music.replace_cover") }}</el-button>
            <el-button :icon="Headset" @click="selectAudio">{{ $t("music.replace_audio") }}</el-button>
            <p class="file-name">
              {{ coverFile ? $t("music.selected_file", { name: coverFile.name }) : $t("music.no_file_selected") }}
            </p>
            <p v-if="coverFile" class="file-size">{{ formatFileSize(coverFile.size) }}</p>
            <p v-if="audioFile" class="file-name">{{ $t("music.selected_file", { name: audioFile.name }) }}</p>
            <p v-if="audioFile" class="file-size">{{ formatFileSize(audioFile.size) }}</p>
          </div>
        </div>

        <input ref="coverInputRef" class="hidden-input" type="file" accept="image/*" @change="handleCoverSelected" />
        <input ref="audioInputRef" class="hidden-input" type="file" accept="audio/*" @change="handleAudioSelected" />

        <el-form ref="formRef" class="edit-form" :model="form" :rules="rules" label-position="top">
          <el-row :gutter="20">
            <el-col :xs="24" :sm="12">
              <el-form-item :label="$t('add.music_title')" prop="title">
                <el-input v-model="form.title" />
              </el-form-item>
            </el-col>
            <el-col :xs="24" :sm="12">
              <el-form-item :label="$t('add.music_artist')" prop="artist">
                <el-input v-model="form.artist" />
              </el-form-item>
            </el-col>
          </el-row>
          <el-row :gutter="20">
            <el-col :xs="24" :sm="12">
              <el-form-item :label="$t('add.music_album')">
                <el-input v-model="form.album" />
              </el-form-item>
            </el-col>
            <el-col :xs="12" :sm="6">
              <el-form-item :label="$t('add.music_year')">
                <el-input-number v-model="form.year" :min="1000" :max="9999" controls-position="right" />
              </el-form-item>
            </el-col>
            <el-col :xs="12" :sm="6">
              <el-form-item :label="$t('add.music_track')">
                <el-input-number v-model="form.trackNumber" :min="1" controls-position="right" />
              </el-form-item>
            </el-col>
          </el-row>
          <el-row :gutter="20">
            <el-col :xs="24" :sm="12">
              <el-form-item :label="$t('add.music_genre')">
                <el-input v-model="form.genre" />
              </el-form-item>
            </el-col>
            <el-col :xs="24" :sm="12">
              <el-form-item :label="$t('add.music_duration')">
                <el-input v-model="form.duration" placeholder="03:45" />
              </el-form-item>
            </el-col>
          </el-row>
          <el-form-item :label="$t('add.music_description')" prop="intro">
            <el-input v-model="form.intro" type="textarea" :rows="4" />
          </el-form-item>

          <div v-if="uploadPercent > 0" class="upload-progress">
            <el-progress :percentage="uploadPercent" :stroke-width="12" text-inside />
          </div>

          <div class="form-actions">
            <el-button type="primary" :icon="UploadFilled" :loading="saving" @click="handleSave(formRef)">
              {{ $t("music.save_changes") }}
            </el-button>
            <el-button :loading="deleting" type="danger" :icon="Delete" @click="handleDelete">
              {{ $t("music.delete_music") }}
            </el-button>
            <el-button @click="router.push(`/music/${id}`)">{{ $t("common.cancel") }}</el-button>
          </div>
        </el-form>
      </div>
    </el-card>
  </div>
</template>

<style scoped lang="scss">
.edit-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: $spacing-lg;
  margin-bottom: $spacing-xl;

  h2 {
    margin: 0 0 $spacing-xs;
    color: var(--text-dark);
  }

  p {
    margin: 0;
    color: var(--text-light);
  }
}

.edit-body {
  display: grid;
  grid-template-columns: minmax(220px, 300px) 1fr;
  gap: $spacing-2xl;
}

.media-panel {
  display: flex;
  flex-direction: column;
  gap: $spacing-md;
}

.cover-preview {
  width: 100%;
  aspect-ratio: 1;
  border-radius: $radius-md;
  overflow: hidden;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.08);
}

.file-actions {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: $spacing-sm;
}

.file-caption {
  font-size: $fs-sm;
  font-weight: $fw-semibold;
  color: var(--text-light);
}

.file-name,
.file-size {
  margin: 0;
  max-width: 100%;
  color: var(--text-light);
  font-size: $fs-sm;
  word-break: break-word;
}

.hidden-input {
  display: none;
}

.edit-form {
  min-width: 0;
}

.upload-progress {
  margin-bottom: $spacing-md;
}

.form-actions {
  display: flex;
  flex-wrap: wrap;
  gap: $spacing-sm;
}

@include mobile {
  .edit-body {
    grid-template-columns: 1fr;
  }

  .edit-header {
    flex-direction: column;
  }
}
</style>
