<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import request from '@/utils/request'
import { ElMessage } from 'element-plus'
import type { AxiosProgressEvent } from 'axios'
import type { UploadInstance, UploadFile } from 'element-plus'
import { Close, UploadFilled, PictureFilled, Headset, QuestionFilled } from '@element-plus/icons-vue'
import { loadCachedMeta, saveCachedMeta, removeCachedMeta, parseAudioFile, formatFileSize } from '@/utils/upload'

const router = useRouter()
useI18n()

const loading = ref(false)
const coverFile = ref<File | null>(null)
const audioFile = ref<File | null>(null)
const uploadPercent = ref(0)
const coverUploadRef = ref<UploadInstance>()
const audioUploadRef = ref<UploadInstance>()

const form = reactive({
  title: '',
  artist: '',
  album: '',
  year: '',
  track: '',
  genre: '',
  duration: '',
  description: '',
})

const touched = reactive({
  title: false,
  artist: false,
  album: false,
  year: false,
  track: false,
  genre: false,
})

const applyMetaToForm = (meta: Record<string, string>) => {
  if (!touched.title && meta.title) form.title = meta.title
  if (!touched.artist && meta.artist) form.artist = meta.artist
  if (!touched.album && meta.album) form.album = meta.album
  if (!touched.year && meta.year) form.year = meta.year
  if (!touched.track && meta.track) form.track = meta.track
  if (!touched.genre && meta.genre) form.genre = meta.genre
  if (meta.duration) form.duration = meta.duration
}

const handleCoverChange = (file: any) => {
  coverFile.value = file?.raw || null
}

const handleCoverExceed = (files: UploadFile[]) => {
  coverUploadRef.value?.clearFiles()
  const raw = files?.[0]?.raw as File | undefined
  if (raw) coverFile.value = raw
}

const removeCover = () => {
  coverFile.value = null
  coverUploadRef.value?.clearFiles()
}

const handleAudioChange = async (file: any) => {
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
    ElMessage.warning('Failed to read audio tags')
  }
}

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
    ElMessage.warning('Failed to read audio tags')
  }
}

const removeAudio = () => {
  audioFile.value = null
  audioUploadRef.value?.clearFiles()
}

const handleSubmit = async () => {
  if (!form.title || !form.artist) {
    ElMessage.error('Title and artist are required')
    return
  }
  loading.value = true
  uploadPercent.value = 0
  try {
    const res: any = await request.post('/musics', {
      title: form.title,
      artist: form.artist,
      intro: form.description,
    })
    const musicId = res.data?.id
    if (!musicId) {
      ElMessage.error('Failed to create music record')
      return
    }

    if (audioFile.value || coverFile.value) {
      const fd = new FormData()
      if (audioFile.value) fd.append('file', audioFile.value)
      if (coverFile.value) fd.append('cover', coverFile.value)
      await request.post(`/musics/${musicId}/upload`, fd, {
        headers: { 'Content-Type': 'multipart/form-data' },
        onUploadProgress: (event: AxiosProgressEvent) => {
          if (!event.total) return
          uploadPercent.value = Math.round((event.loaded / event.total) * 100)
        },
      })
    }

    if (audioFile.value) removeCachedMeta(audioFile.value)
    ElMessage.success('Music uploaded successfully')
    coverFile.value = null
    audioFile.value = null
    coverUploadRef.value?.clearFiles()
    audioUploadRef.value?.clearFiles()
    Object.assign(form, { title: '', artist: '', album: '', year: '', track: '', genre: '', duration: '', description: '' })
  } catch (_e) {
  } finally {
    loading.value = false
    setTimeout(() => { uploadPercent.value = 0 }, 800)
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
            <p class="file-card-label">{{ $t('add.cover') }}</p>
            <p class="file-card-name">{{ coverFile ? coverFile.name : 'Not selected' }}</p>
            <p v-if="coverFile" class="file-card-size">{{ formatFileSize(coverFile.size) }}</p>
          </div>
        </div>
        <button v-if="coverFile" class="dismiss-btn" @click="removeCover" :aria-label="$t('common.delete')">
          <el-icon :size="14"><Close /></el-icon>
        </button>
        <el-upload v-else class="card-upload-overlay" ref="coverUploadRef" :show-file-list="false" :limit="1"
          :auto-upload="false" :on-change="handleCoverChange" :on-exceed="handleCoverExceed" accept="image/*">
          <el-icon :size="20"><UploadFilled /></el-icon>
        </el-upload>
      </div>

      <div class="file-card" :class="{ filled: audioFile }">
        <div class="file-card-body">
          <el-icon :size="28"><Headset /></el-icon>
          <div class="file-card-info">
            <p class="file-card-label">{{ $t('add.audio') }}</p>
            <p class="file-card-name">{{ audioFile ? audioFile.name : 'Not selected' }}</p>
            <p v-if="audioFile" class="file-card-size">{{ formatFileSize(audioFile.size) }}</p>
          </div>
        </div>
        <button v-if="audioFile" class="dismiss-btn" @click="removeAudio" :aria-label="$t('common.delete')">
          <el-icon :size="14"><Close /></el-icon>
        </button>
        <el-upload v-else class="card-upload-overlay" ref="audioUploadRef" :show-file-list="false" :limit="1"
          :auto-upload="false" :on-change="handleAudioChange" :on-exceed="handleAudioExceed" accept="audio/*">
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
              {{ $t('add.music_track') }}
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
          {{ $t('common.upload') }}
        </el-button>
        <el-button size="large" @click="router.back()">{{ $t('common.cancel') }}</el-button>
      </el-form-item>

      <div v-if="uploadPercent > 0" class="upload-progress">
        <el-progress :percentage="uploadPercent" :stroke-width="12" :text-inside="true" />
      </div>
    </el-form>
  </div>
</template>

<style scoped>
.file-cards {
  display: flex;
  gap: 16px;
  margin-bottom: 24px;
}

.file-card {
  flex: 1;
  position: relative;
  min-height: 88px;
  border: 2px dashed var(--border-color);
  border-radius: 12px;
  padding: 16px;
  display: flex;
  align-items: center;
  transition: border-color 0.2s, background 0.2s;
  background: var(--bg-white);
}

.file-card.filled {
  border-style: solid;
  border-color: var(--accent-color);
  background: color-mix(in srgb, var(--accent-color) 4%, var(--bg-white));
}

.file-card:hover {
  border-color: var(--accent-color);
}

.file-card-body {
  display: flex;
  align-items: center;
  gap: 12px;
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
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: var(--text-light);
  margin: 0 0 2px;
}

.file-card-name {
  margin: 0;
  font-size: 0.9rem;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.file-card-size {
  margin: 2px 0 0;
  font-size: 0.78rem;
  color: var(--text-light);
}

.dismiss-btn {
  position: absolute;
  top: -8px;
  right: -8px;
  width: 24px;
  height: 24px;
  border-radius: 50%;
  border: 2px solid var(--bg-white);
  background: #ef4444;
  color: #fff;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  transition: transform 0.15s, box-shadow 0.15s;
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.15);
  z-index: 2;
}

.dismiss-btn:hover {
  transform: scale(1.15);
  box-shadow: 0 4px 12px rgba(239, 68, 68, 0.4);
}

.card-upload-overlay {
  position: absolute;
  top: 50%;
  right: 16px;
  transform: translateY(-50%);
  color: var(--text-light);
  cursor: pointer;
  transition: color 0.2s;
}

.card-upload-overlay:hover {
  color: var(--accent-color);
}

.upload-progress {
  margin-top: 8px;
}

.help-icon {
  color: var(--text-light);
  margin-left: 4px;
  cursor: help;
}

@media (max-width: 640px) {
  .file-cards {
    flex-direction: column;
  }
}
</style>
