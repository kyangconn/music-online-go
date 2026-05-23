<script setup lang="ts">
import type { AxiosProgressEvent } from "axios"
import type { UploadInstance, UploadFile } from "element-plus"
import { Close, UploadFilled, PictureFilled, Headset, QuestionFilled } from "@element-plus/icons-vue"
import { ElMessage } from "element-plus"
import { ref, reactive } from "vue"
import { useI18n } from "vue-i18n"
import { useRouter } from "vue-router"
import type { CreateMusicData, MusicMetadataFields } from "@/types/api"
import request from "@/utils/request"
import { loadCachedMeta, saveCachedMeta, removeCachedMeta, parseAudioFile, formatFileSize } from "@/utils/upload"

const router = useRouter()
useI18n()

const loading = ref(false)
const coverFile = ref<File | null>(null)
const audioFile = ref<File | null>(null)
const uploadPercent = ref(0)
const coverUploadRef = ref<UploadInstance>()
const audioUploadRef = ref<UploadInstance>()

const form = reactive({
  title: "",
  artist: "",
  album: "",
  year: "",
  track: "",
  genre: "",
  duration: "",
  description: "",
})

const touched = reactive({
  title: false,
  artist: false,
  album: false,
  year: false,
  track: false,
  genre: false,
})

/** 将解析的元数据填入表单 */
const applyMetaToForm = (meta: MusicMetadataFields) => {
  if (!touched.title && meta.title) form.title = meta.title
  if (!touched.artist && meta.artist) form.artist = meta.artist
  if (!touched.album && meta.album) form.album = meta.album
  if (!touched.year && meta.year) form.year = meta.year
  if (!touched.track && meta.track) form.track = meta.track
  if (!touched.genre && meta.genre) form.genre = meta.genre
  if (meta.duration) form.duration = meta.duration
}

/** 封面文件选择回调 */
const handleCoverChange = (file: UploadFile) => {
  coverFile.value = file?.raw || null
}

const handleCoverExceed = (files: UploadFile[]) => {
  coverUploadRef.value?.clearFiles()
  const raw = files?.[0]?.raw as File | undefined
  if (raw) coverFile.value = raw
}

/** 移除封面 */
const removeCover = () => {
  coverFile.value = null
  coverUploadRef.value?.clearFiles()
}

/** 音频文件选择回调，自动解析标签 */
const handleAudioChange = async (file: UploadFile) => {
  audioFile.value = file?.raw || null
  if (!audioFile.value) return

  const cached = loadCachedMeta(audioFile.value)
  if (cached) {
    applyMetaToForm(cached)
    return
  }

  try {
    const meta = await parseAudioFile(audioFile.value)
    saveCachedMeta(audioFile.value, meta)
    applyMetaToForm(meta)
  } catch (_e) {
    ElMessage.warning("Failed to read audio tags")
  }
}

/** 音频文件超限处理 */
const handleAudioExceed = async (files: UploadFile[]) => {
  audioUploadRef.value?.clearFiles()
  const raw = files?.[0]?.raw as File | undefined
  if (!raw) return
  audioFile.value = raw

  const cached = loadCachedMeta(raw)
  if (cached) {
    applyMetaToForm(cached)
    return
  }

  try {
    const meta = await parseAudioFile(raw)
    saveCachedMeta(raw, meta)
    applyMetaToForm(meta)
  } catch (_e) {
    ElMessage.warning("Failed to read audio tags")
  }
}

/** 移除音频 */
const removeAudio = () => {
  audioFile.value = null
  audioUploadRef.value?.clearFiles()
}

/** 提交音乐创建并上传文件 */
const handleSubmit = async () => {
  if (!form.title || !form.artist) {
    ElMessage.error("Title and artist are required")
    return
  }
  loading.value = true
  uploadPercent.value = 0
  try {
    const res = await request.post<CreateMusicData>("/musics", {
      title: form.title,
      artist: form.artist,
      intro: form.description,
    })
    const musicId = res.data?.id
    if (!musicId) {
      ElMessage.error("Failed to create music record")
      return
    }

    if (audioFile.value || coverFile.value) {
      const fd = new FormData()
      if (audioFile.value) fd.append("file", audioFile.value)
      if (coverFile.value) fd.append("cover", coverFile.value)
      await request.post(`/musics/${musicId}/upload`, fd, {
        headers: { "Content-Type": "multipart/form-data" },
        onUploadProgress: (event: AxiosProgressEvent) => {
          if (!event.total) return
          uploadPercent.value = Math.round((event.loaded / event.total) * 100)
        },
      })
    }

    if (audioFile.value) removeCachedMeta(audioFile.value)
    ElMessage.success("Music uploaded successfully")
    coverFile.value = null
    audioFile.value = null
    coverUploadRef.value?.clearFiles()
    audioUploadRef.value?.clearFiles()
    Object.assign(form, {
      title: "",
      artist: "",
      album: "",
      year: "",
      track: "",
      genre: "",
      duration: "",
      description: "",
    })
  } catch (_e) {
  } finally {
    loading.value = false
    setTimeout(() => {
      uploadPercent.value = 0
    }, 800)
  }
}
</script>

<template>
  <div class="single-upload">
    <div class="file-cards">
      <div class="file-card" :class="{ filled: coverFile }">
        <div class="file-card-body">
          <el-icon :size="28"><PictureFilled /></el-icon>
          <div class="file-card-info">
            <p class="file-card-label">{{ $t("add.cover") }}</p>
            <p class="file-card-name">{{ coverFile ? coverFile.name : "Not selected" }}</p>
            <p v-if="coverFile" class="file-card-size">{{ formatFileSize(coverFile.size) }}</p>
          </div>
        </div>
        <button v-if="coverFile" class="dismiss-btn" @click="removeCover" :aria-label="$t('common.delete')">
          <el-icon :size="14"><Close /></el-icon>
        </button>
        <el-upload
          v-else
          class="card-upload-overlay"
          ref="coverUploadRef"
          :show-file-list="false"
          :limit="1"
          :auto-upload="false"
          :on-change="handleCoverChange"
          :on-exceed="handleCoverExceed"
          accept="image/*"
        >
          <el-icon :size="20"><UploadFilled /></el-icon>
        </el-upload>
      </div>

      <div class="file-card" :class="{ filled: audioFile }">
        <div class="file-card-body">
          <el-icon :size="28"><Headset /></el-icon>
          <div class="file-card-info">
            <p class="file-card-label">{{ $t("add.audio") }}</p>
            <p class="file-card-name">{{ audioFile ? audioFile.name : "Not selected" }}</p>
            <p v-if="audioFile" class="file-card-size">{{ formatFileSize(audioFile.size) }}</p>
          </div>
        </div>
        <button v-if="audioFile" class="dismiss-btn" @click="removeAudio" :aria-label="$t('common.delete')">
          <el-icon :size="14"><Close /></el-icon>
        </button>
        <el-upload
          v-else
          class="card-upload-overlay"
          ref="audioUploadRef"
          :show-file-list="false"
          :limit="1"
          :auto-upload="false"
          :on-change="handleAudioChange"
          :on-exceed="handleAudioExceed"
          accept="audio/*"
        >
          <el-icon :size="20"><UploadFilled /></el-icon>
        </el-upload>
      </div>
    </div>

    <el-form label-position="top" :model="form">
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
            <el-input type="textarea" v-model="form.description" :rows="2" />
          </el-form-item>
        </el-col>
      </el-row>

      <el-form-item>
        <el-button type="primary" :loading="loading" size="large" @click="handleSubmit">
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

.file-card {
  flex: 1;
  position: relative;
  min-height: 88px;
  border: 2px dashed var(--border-color);
  border-radius: $radius-xl;
  padding: $spacing-lg;
  @include inline-flex;
  transition:
    border-color $transition-base,
    background $transition-base;
  background: var(--bg-white);

  &.filled {
    border-style: solid;
    border-color: var(--accent-color);
    background: color-mix(in srgb, var(--accent-color) 4%, var(--bg-white));
  }
  &:hover {
    border-color: var(--accent-color);
  }
}

.file-card-body {
  @include inline-flex($spacing-md);
  flex: 1;
  color: var(--text-secondary);
}

.file-card.filled .file-card-body {
  color: var(--text-primary);
}

.file-card-info {
  min-width: 0;
}

.file-card-label {
  font-size: 0.8rem;
  font-weight: $fw-semibold;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: var(--text-light);
  margin: 0 0 $spacing-xs;
}

.file-card-name {
  margin: 0;
  font-size: $fs-base;
  @include text-ellipsis;
}

.file-card-size {
  margin: $spacing-xs 0 0;
  font-size: $fs-xs;
  color: var(--text-light);
}

.dismiss-btn {
  position: absolute;
  top: -8px;
  right: -8px;
  width: $spacing-2xl;
  height: $spacing-2xl;
  border-radius: $radius-round;
  border: 2px solid var(--bg-white);
  background: $color-danger;
  color: #fff;
  cursor: pointer;
  @include flex-center;
  padding: 0;
  transition:
    transform 0.15s,
    box-shadow 0.15s;
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.15);
  z-index: 2;

  &:hover {
    transform: scale(1.15);
    box-shadow: 0 4px 12px rgba($color-danger, 0.4);
  }
}

.card-upload-overlay {
  position: absolute;
  top: 50%;
  right: $spacing-lg;
  transform: translateY(-50%);
  color: var(--text-light);
  cursor: pointer;
  transition: color $transition-base;

  &:hover {
    color: var(--accent-color);
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
