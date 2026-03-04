<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import request from '@/utils/request'
import { ElMessage } from 'element-plus'
import type { AxiosProgressEvent } from 'axios'
import { parseBlob } from 'music-metadata-browser'
import type { UploadInstance, UploadFile } from 'element-plus'
import { FolderOpened } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const loading = ref(false)
const activeTab = ref('single')
const directoryHandle = ref<any>(null) // 重新引入 directoryHandle
const allScannedFiles = ref<any[]>([]) // 存储所有扫描到的文件
const currentPage = ref(1)
const pageSize = 10 // 每页显示的文件数量
const fileScanLimit = 500 // 最大扫描文件数量，防止一次性加载过多
const scanDelayMs = 10 // 扫描每个文件后的延迟（毫秒）

const paginatedFiles = computed(() => {
  const start = (currentPage.value - 1) * pageSize
  const end = start + pageSize
  return allScannedFiles.value.slice(start, end)
})

const selectedFiles = ref<Set<string>>(new Set())
const parsing = ref(false)
const batchUploading = ref(false)
const batchProgress = ref(0)
const coverFile = ref<File | null>(null)
const audioFile = ref<File | null>(null)
const uploadPercent = ref(0)
const coverUploadRef = ref<UploadInstance>()
const audioUploadRef = ref<UploadInstance>()
const supportsFSAccess = ref(false)
const directoryInputRef = ref<HTMLInputElement>()

const form = reactive({
  title: '',
  artist: '',
  album: '',
  year: '',
  track: '',
  genre: '',
  duration: '',
  description: ''
})

const touched = reactive({
  title: false,
  artist: false,
  album: false,
  year: false,
  track: false,
  genre: false
})

const handleCoverChange = (file: any) => {
  coverFile.value = file?.raw || null
}

const handleCoverExceed = (files: UploadFile[]) => {
  coverUploadRef.value?.clearFiles()
  const raw = files?.[0]?.raw as File | undefined
  if (raw) {
    coverFile.value = raw
  }
}

const handleAudioChange = async (file: any) => {
  audioFile.value = file?.raw || null

  if (!audioFile.value) return

  try {
    const metadata = await parseBlob(audioFile.value)
    const common = metadata.common

    if (!touched.title && common.title) {
      form.title = common.title
    }
    if (!touched.artist && common.artist) {
      form.artist = common.artist
    }
    if (!touched.album && common.album) {
      form.album = common.album
    }
    if (!touched.year && typeof common.year === 'number') {
      form.year = String(common.year)
    }
    if (!touched.track && common.track && typeof common.track.no === 'number') {
      form.track = String(common.track.no)
    }
    if (!touched.genre && common.genre && common.genre.length > 0) {
      form.genre = common.genre.join('; ')
    }
  } catch (e) {
    console.error(e)
    ElMessage.warning('Failed to read audio tags')
  }
}

const handleAudioExceed = async (files: UploadFile[]) => {
  audioUploadRef.value?.clearFiles()
  const raw = files?.[0]?.raw as File | undefined
  if (!raw) return
  audioFile.value = raw
  try {
    const metadata = await parseBlob(raw)
    const common = metadata.common
    if (!touched.title && common.title) form.title = common.title
    if (!touched.artist && common.artist) form.artist = common.artist
    if (!touched.album && common.album) form.album = common.album
    if (!touched.year && typeof common.year === 'number') form.year = String(common.year)
    if (!touched.track && common.track && typeof common.track.no === 'number') form.track = String(common.track.no)
    if (!touched.genre && common.genre && common.genre.length > 0) form.genre = common.genre.join('; ')
  } catch (e) {
    ElMessage.warning('Failed to read audio tags')
  }
}

const resetUploads = () => {
  coverUploadRef.value?.clearFiles()
  audioUploadRef.value?.clearFiles()
  coverFile.value = null
  audioFile.value = null
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
    fd.append('year', form.year)
    fd.append('track', form.track)
    fd.append('genre', form.genre)
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
    resetUploads()
  } catch (e) {
  } finally {
    loading.value = false
    setTimeout(() => {
      uploadPercent.value = 0
    }, 800)
  }
}

// 本地文件批量导入功能
const requestDirectoryAccess = async () => {
  if (supportsFSAccess.value) {
    // 使用 File System Access API
    parsing.value = true
    allScannedFiles.value = [] // 清空之前扫描的文件
    currentPage.value = 1 // 重置页码
    try {
      const handle = await (window as any).showDirectoryPicker({
        mode: 'read'
      })
      directoryHandle.value = handle
      await scanDirectory(handle)

      if (allScannedFiles.value.length >= fileScanLimit) {
        ElMessage.warning(`Scan stopped at ${fileScanLimit} files. Displaying first ${pageSize} files.`)
      } else if (allScannedFiles.value.length > 0) {
        ElMessage.success(`Found ${allScannedFiles.value.length} audio file(s). Displaying first ${pageSize} files.`)
      } else {
        ElMessage.info('No audio files found in the selected directory.')
      }

    } catch (error: any) {
      if (error.name === 'AbortError') {
        // 用户取消选择
      } else {
        ElMessage.error(error?.message || t('settings.local_access_error'))
      }
    } finally {
      parsing.value = false
    }
  } else {
    // 使用 webkitdirectory 回退
    directoryInputRef.value?.click()
  }
}

// 处理 webkitdirectory 输入变化
const handleDirectoryInputChange = async (event: Event) => {
  const input = event.target as HTMLInputElement
  const files = input.files
  if (!files || files.length === 0) {
    return
  }

  parsing.value = true
  allScannedFiles.value = [] // 清空之前扫描的文件
  currentPage.value = 1 // 重置页码

  // 将 FileList 转换为数组
  const fileArray = Array.from(files)

  for (const file of fileArray) {
    if (allScannedFiles.value.length >= fileScanLimit) {
      break // 达到文件扫描限制，停止扫描
    }

    const name = file.name.toLowerCase()
    if (name.endsWith('.mp3') || name.endsWith('.wav') || name.endsWith('.flac') ||
      name.endsWith('.ogg') || name.endsWith('.m4a') || name.endsWith('.aac')) {

      // 使用 webkitRelativePath 作为路径
      const relativePath = (file as any).webkitRelativePath || file.name
      allScannedFiles.value.push({
        handle: null, // 没有目录句柄
        file,
        name: file.name,
        path: relativePath,
        size: file.size,
        type: file.type,
        metadata: null,
        loading: false,
        selected: false
      })

      // 添加延迟，避免UI卡顿
      await new Promise(resolve => setTimeout(resolve, scanDelayMs))
    }
  }

  parsing.value = false

  if (allScannedFiles.value.length >= fileScanLimit) {
    ElMessage.warning(`Scan stopped at ${fileScanLimit} files. Displaying first ${pageSize} files.`)
  } else if (allScannedFiles.value.length > 0) {
    ElMessage.success(`Found ${allScannedFiles.value.length} audio file(s). Displaying first ${pageSize} files.`)
  } else {
    ElMessage.info('No audio files found in the selected directory.')
  }

  // 清空 input 值，允许重复选择同一目录
  input.value = ''
}

const scanDirectory = async (dirHandle: any, path = '') => {
  for await (const entry of dirHandle.values()) {
    if (allScannedFiles.value.length >= fileScanLimit) {
      return // 达到文件扫描限制，停止扫描
    }

    if (entry.kind === 'file') {
      const name = entry.name.toLowerCase()
      if (name.endsWith('.mp3') || name.endsWith('.wav') || name.endsWith('.flac') ||
        name.endsWith('.ogg') || name.endsWith('.m4a') || name.endsWith('.aac')) {
        const file = await entry.getFile()
        allScannedFiles.value.push({
          handle: entry,
          file,
          name: entry.name,
          path: path ? `${path}/${entry.name}` : entry.name,
          size: file.size,
          type: file.type,
          metadata: null,
          loading: false,
          selected: false
        })
        // 添加延迟，避免UI卡顿
        await new Promise(resolve => setTimeout(resolve, scanDelayMs))
      }
    } else if (entry.kind === 'directory') {
      await scanDirectory(entry, path ? `${path}/${entry.name}` : entry.name)
    }
  }
}

const toggleFileSelection = (path: string) => {
  if (selectedFiles.value.has(path)) {
    selectedFiles.value.delete(path)
  } else {
    selectedFiles.value.add(path)
  }
}

const parseFileMetadata = async (fileItem: any) => {
  if (fileItem.metadata) return

  fileItem.loading = true
  try {
    const metadata = await parseBlob(fileItem.file)
    fileItem.metadata = {
      title: metadata.common.title || '',
      artist: metadata.common.artist || '',
      album: metadata.common.album || '',
      year: metadata.common.year || '',
      track: metadata.common.track?.no || '',
      genre: metadata.common.genre?.join('; ') || '',
      duration: metadata.format.duration ? Math.round(metadata.format.duration) : 0
    }
  } catch (error) {
    console.error('Failed to parse metadata:', error)
    fileItem.metadata = {
      title: '',
      artist: '',
      album: '',
      year: '',
      track: '',
      genre: '',
      duration: 0
    }
  } finally {
    fileItem.loading = false
  }
}

const parseAllSelectedMetadata = async () => {
  parsing.value = true
  const selectedItems = paginatedFiles.value.filter(item => selectedFiles.value.has(item.path))

  for (const item of selectedItems) {
    if (!item.metadata) {
      await parseFileMetadata(item)
      // 添加延迟避免UI冻结
      await new Promise(resolve => setTimeout(resolve, 50))
    }
  }

  parsing.value = false
  ElMessage.success(`Parsed metadata for ${selectedItems.length} file(s).`)
}

const uploadSelectedFiles = async () => {
  if (selectedFiles.value.size === 0) {
    ElMessage.warning('Please select files to upload.')
    return
  }

  batchUploading.value = true
  batchProgress.value = 0

  const selectedItems = allScannedFiles.value.filter(item => selectedFiles.value.has(item.path))
  const total = selectedItems.length
  let completed = 0

  for (const item of selectedItems) {
    try {
      // 创建FormData
      const fd = new FormData()
      const metadata = item.metadata || {}

      fd.append('title', metadata.title || item.name.replace(/\.[^/.]+$/, ''))
      fd.append('artist', metadata.artist || '')
      fd.append('album', metadata.album || '')
      fd.append('year', metadata.year || '')
      fd.append('track', metadata.track || '')
      fd.append('genre', metadata.genre || '')
      fd.append('duration', metadata.duration ? `${Math.floor(metadata.duration / 60)}:${String(metadata.duration % 60).padStart(2, '0')}` : '')
      fd.append('description', '')
      fd.append('path', `local:${item.path}`)
      fd.append('file', item.file)

      await request.post('/musics', fd, {
        headers: { 'Content-Type': 'multipart/form-data' }
      })

      completed++
      batchProgress.value = Math.round((completed / total) * 100)
    } catch (error) {
      console.error(`Failed to upload ${item.name}:`, error)
      ElMessage.error(`Failed to upload ${item.name}`)
    }
  }

  batchUploading.value = false
  if (completed > 0) {
    ElMessage.success(`Successfully uploaded ${completed} of ${total} file(s).`)
  }
  selectedFiles.value.clear()
  batchProgress.value = 0
}

const selectAllFiles = () => {
  const allSelectedOnPage = paginatedFiles.value.every(item => selectedFiles.value.has(item.path))
  if (allSelectedOnPage) {
    paginatedFiles.value.forEach(item => {
      selectedFiles.value.delete(item.path)
    })
  } else {
    paginatedFiles.value.forEach(item => {
      selectedFiles.value.add(item.path)
    })
  }
}

// 辅助函数
const formatFileSize = (bytes: number) => {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

const formatDuration = (seconds: number) => {
  const mins = Math.floor(seconds / 60)
  const secs = Math.floor(seconds % 60)
  return `${mins}:${secs.toString().padStart(2, '0')}`
}

onMounted(() => {
  supportsFSAccess.value = !!(window as any).showDirectoryPicker
})
</script>

<template>
  <div class="add-container">
    <el-card class="add-card" shadow="never">
      <el-tabs v-model="activeTab">
        <el-tab-pane :label="$t('add.single_import')" name="single">
          <el-row :gutter="20">
            <el-col :xs="24" :sm="10" :md="8" :lg="6">
              <div class="upload-section">
                <h3>{{ $t('add.cover') }}</h3>
                <el-upload class="upload-block" ref="coverUploadRef" :show-file-list="false" :limit="1"
                  :auto-upload="false" :on-change="handleCoverChange" :on-exceed="handleCoverExceed" accept="image/*">
                  <el-button>{{ $t('add.import_cover') }}</el-button>
                </el-upload>
                <h3 style="margin-top:16px">{{ $t('add.audio') }}</h3>
                <el-upload class="upload-block" ref="audioUploadRef" :show-file-list="false" :limit="1"
                  :auto-upload="false" :on-change="handleAudioChange" :on-exceed="handleAudioExceed" accept="audio/*">
                  <el-button>{{ $t('add.import') }}</el-button>
                </el-upload>
                <div class="file-names">
                  <p v-if="coverFile">Cover: {{ coverFile.name }}</p>
                  <p v-if="audioFile">Audio: {{ audioFile.name }}</p>
                </div>
              </div>
            </el-col>
            <el-col :xs="24" :sm="14" :md="16" :lg="18">
              <el-form label-position="top" :model="form">
                <el-form-item :label="$t('add.music_title')">
                  <el-input v-model="form.title" @input="touched.title = true" />
                </el-form-item>
                <el-form-item :label="$t('add.music_artist')">
                  <el-input v-model="form.artist" @input="touched.artist = true" />
                </el-form-item>
                <el-form-item :label="$t('add.music_album')">
                  <el-input v-model="form.album" @input="touched.album = true" />
                </el-form-item>
                <el-form-item :label="$t('add.music_year')">
                  <el-input v-model="form.year" placeholder="e.g. 2024" @input="touched.year = true" />
                </el-form-item>
                <el-form-item :label="$t('add.music_track')">
                  <el-input v-model="form.track" placeholder="e.g. 1" @input="touched.track = true" />
                </el-form-item>
                <el-form-item :label="$t('add.music_genre')">
                  <el-input v-model="form.genre" placeholder="e.g. Rock; Alternative" @input="touched.genre = true" />
                </el-form-item>
                <el-form-item :label="$t('add.music_duration')">
                  <el-input v-model="form.duration" placeholder="e.g. 03:45" />
                </el-form-item>
                <el-form-item :label="$t('add.music_description')">
                  <el-input type="textarea" v-model="form.description" />
                </el-form-item>
                <el-form-item>
                  <el-button type="primary" :loading="loading" @click="handleSubmit">{{ $t('common.upload') }}</el-button>
                  <el-button @click="$router.back()">{{ $t('common.cancel') }}</el-button>
                </el-form-item>
                <el-form-item v-if="uploadPercent > 0">
                  <el-progress :percentage="uploadPercent" :stroke-width="14" />
                </el-form-item>
              </el-form>
            </el-col>
          </el-row>
        </el-tab-pane>

        <el-tab-pane :label="$t('add.batch_import')" name="batch">
          <div class="batch-import-container">
            <div class="batch-controls">
              <input ref="directoryInputRef" v-if="!supportsFSAccess" type="file" webkitdirectory multiple
                @change="handleDirectoryInputChange" style="display: none;" />
              <el-button type="primary" :loading="parsing" @click="requestDirectoryAccess" :disabled="directoryHandle">
                {{ $t('add.batch_import_desc') }}
              </el-button>

              <div class="batch-actions" v-if="allScannedFiles.length > 0">
                <el-button @click="selectAllFiles">
                  {{ selectedFiles.size === allScannedFiles.length ? 'Deselect All' : 'Select All' }}
                </el-button>
                <el-button type="primary" plain :loading="parsing" @click="parseAllSelectedMetadata"
                  :disabled="selectedFiles.size === 0">
                  Parse Metadata
                </el-button>
                <el-button type="success" :loading="batchUploading" @click="uploadSelectedFiles"
                  :disabled="selectedFiles.size === 0">
                  Upload Selected ({{ selectedFiles.size }})
                </el-button>
              </div>
            </div>

            <div v-if="batchProgress > 0" class="batch-progress">
              <el-progress :percentage="batchProgress" :stroke-width="14" />
              <p>Uploading... {{ batchProgress }}%</p>
            </div>

            <div class="files-list" v-if="allScannedFiles.length > 0">
              <el-table :data="paginatedFiles" style="width: 100%" size="small">
                <el-table-column width="50">
                  <template #default="{ row }">
                    <el-checkbox :model-value="selectedFiles.has(row.path)" @change="toggleFileSelection(row.path)" />
                  </template>
                </el-table-column>
                <el-table-column prop="name" :label="$t('add.file_name')" min-width="200">
                  <template #default="{ row }">
                    <div class="file-name-cell">
                      <span>{{ row.name }}</span>
                      <el-button v-if="!row.metadata" size="small" :loading="row.loading"
                        @click="parseFileMetadata(row)">
                        Parse
                      </el-button>
                    </div>
                  </template>
                </el-table-column>
                <el-table-column prop="size" :label="$t('add.size')" width="100">
                  <template #default="{ row }">
                    {{ formatFileSize(row.size) }}
                  </template>
                </el-table-column>
                <el-table-column prop="metadata.duration" :label="$t('add.music_duration')" width="100">
                  <template #default="{ row }">
                    {{ row.metadata?.duration ? formatDuration(row.metadata.duration) : 'N/A' }}
                  </template>
                </el-table-column>
                <el-table-column prop="metadata.artist" :label="$t('add.music_artist')" width="150">
                  <template #default="{ row }">
                    {{ row.metadata?.artist || 'N/A' }}
                  </template>
                </el-table-column>
                <el-table-column prop="metadata.title" :label="$t('add.music_title')" width="150">
                  <template #default="{ row }">
                    {{ row.metadata?.title || 'N/A' }}
                  </template>
                </el-table-column>
              </el-table>
              <div class="files-stats">
                Total: {{ allScannedFiles.length }} files | Selected: {{ selectedFiles.size }}
              </div>
              <el-pagination v-model:current-page="currentPage" :page-size="pageSize" :total="allScannedFiles.length"
                layout="prev, pager, next" background small class="mt-4" />
            </div>
            <div class="empty-state" v-else>
              <el-icon :size="50">
                <FolderOpened />
              </el-icon>
              <p>No audio files found or directory not selected.</p>
              <p class="hint">Click "Select Directory" to start importing.</p>
            </div>
          </div>
        </el-tab-pane>
      </el-tabs>
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
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.06);
}

.upload-block {
  width: 100%;
}

.file-names {
  margin-top: 8px;
  font-size: 0.85rem;
  color: var(--text-light);
}

/* 批量导入样式 */
.batch-import-container {
  padding: 20px 0;
}

.batch-controls {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  align-items: center;
  margin-bottom: 20px;
  padding-bottom: 20px;
  border-bottom: 1px solid var(--border-color);
}

.batch-actions {
  display: flex;
  gap: 8px;
  margin-left: auto;
}

.batch-progress {
  margin-bottom: 20px;
  padding: 16px;
  background: var(--bg-white);
  border-radius: 8px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.05);
}

.batch-progress p {
  margin-top: 8px;
  font-size: 0.9rem;
  color: var(--text-light);
}

.files-list {
  background: var(--bg-white);
  border-radius: 8px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.05);
  overflow: hidden;
}

.file-name-cell {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.files-stats {
  padding: 12px 16px;
  border-top: 1px solid var(--border-color);
  font-size: 0.9rem;
  color: var(--text-light);
  text-align: center;
}

.empty-state {
  padding: 60px 20px;
  text-align: center;
  color: var(--text-light);
  background: var(--bg-white);
  border-radius: 8px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.05);
}

.empty-state .hint {
  margin-top: 8px;
  font-size: 0.85rem;
  color: var(--text-lighter);
}
</style>
