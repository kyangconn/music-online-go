<script setup lang="ts">
import { ref, reactive } from 'vue'
import request from '@/utils/request'
import { ElMessage } from 'element-plus'
import type { AxiosProgressEvent } from 'axios'

const loading = ref(false)
const coverFile = ref<File | null>(null)
const audioFile = ref<File | null>(null)
const uploadPercent = ref(0)

const form = reactive({
  title: '',
  artist: '',
  album: '',
  duration: '',
  description: ''
})

const handleCoverChange = (file: any) => {
  coverFile.value = file?.raw || null
}
const handleAudioChange = (file: any) => {
  audioFile.value = file?.raw || null
}

const handleSubmit = async () => {
  if (!audioFile.value) {
    ElMessage.error('Please choose an audio file')
    return
  }
  loading.value = true
  uploadPercent.value = 0
  try {
    const fd = new FormData()
    fd.append('title', form.title)
    fd.append('artist', form.artist)
    fd.append('album', form.album)
    fd.append('duration', form.duration)
    fd.append('description', form.description)
    if (coverFile.value) fd.append('cover', coverFile.value)
    if (audioFile.value) fd.append('file', audioFile.value)
    await request.post('/musics', fd, {
      headers: { 'Content-Type': 'multipart/form-data' },
      onUploadProgress: (event: AxiosProgressEvent) => {
        if (!event.total) return
        uploadPercent.value = Math.round((event.loaded / event.total) * 100)
      }
    })
    ElMessage.success('Music uploaded successfully')
  } catch (e) {
  } finally {
    loading.value = false
    setTimeout(() => {
      uploadPercent.value = 0
    }, 800)
  }
}
</script>

<template>
  <div class="add-container">
    <el-card class="add-card" shadow="never">
      <el-row :gutter="20">
        <el-col :xs="24" :sm="10" :md="8" :lg="6">
          <div class="upload-section">
            <h3>Cover Image</h3>
            <el-upload
              class="upload-block"
              :show-file-list="false"
              :limit="1"
              :auto-upload="false"
              :on-change="handleCoverChange"
              accept="image/*"
            >
              <el-button>Choose Cover</el-button>
            </el-upload>
            <h3 style="margin-top:16px">Audio File</h3>
            <el-upload
              class="upload-block"
              :show-file-list="false"
              :limit="1"
              :auto-upload="false"
              :on-change="handleAudioChange"
              accept="audio/*"
            >
              <el-button>Choose Audio</el-button>
            </el-upload>
            <div class="file-names">
              <p v-if="coverFile">Cover: {{ coverFile.name }}</p>
              <p v-if="audioFile">Audio: {{ audioFile.name }}</p>
            </div>
          </div>
        </el-col>
        <el-col :xs="24" :sm="14" :md="16" :lg="18">
          <el-form label-position="top" :model="form">
            <el-form-item label="Title">
              <el-input v-model="form.title" />
            </el-form-item>
            <el-form-item label="Artist">
              <el-input v-model="form.artist" />
            </el-form-item>
            <el-form-item label="Album">
              <el-input v-model="form.album" />
            </el-form-item>
            <el-form-item label="Duration">
              <el-input v-model="form.duration" placeholder="e.g. 03:45" />
            </el-form-item>
            <el-form-item label="Description">
              <el-input type="textarea" v-model="form.description" />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" :loading="loading" @click="handleSubmit">Submit</el-button>
              <el-button @click="$router.back()">Cancel</el-button>
            </el-form-item>
            <el-form-item v-if="uploadPercent > 0">
              <el-progress :percentage="uploadPercent" :stroke-width="14" />
            </el-form-item>
          </el-form>
        </el-col>
      </el-row>
    </el-card>
  </div>
</template>

<style scoped>
.add-container {
  padding: 20px 0;
}
.upload-section {
  background: var(--bg-white);
  padding: 16px;
  border-radius: 8px;
  box-shadow: 0 4px 12px rgba(0,0,0,0.06);
}
.upload-block {
  width: 100%;
}
.file-names {
  margin-top: 8px;
  font-size: 0.85rem;
  color: var(--text-light);
}
</style>
