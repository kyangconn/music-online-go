<script setup lang="ts">
import { FolderOpened } from "@element-plus/icons-vue";
import { ElMessage } from "element-plus";
import { ref, computed, onMounted } from "vue";
import { useI18n } from "vue-i18n";
import type { ScannedFileItem, CreateMusicData } from "@/types/api";
import request from "@/utils/request";
import { parseAudioFile, formatFileSize, formatDuration } from "@/utils/upload";

const { t } = useI18n();

const directoryHandle = ref<FileSystemDirectoryHandle | null>(null);
const allScannedFiles = ref<ScannedFileItem[]>([]);
const currentPage = ref(1);
const pageSize = ref(10);
const fileScanLimit = 500;
const scanDelayMs = 10;
const supportsFSAccess = ref(false);
const directoryInputRef = ref<HTMLInputElement>();

const paginatedFiles = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value;
  return allScannedFiles.value.slice(start, start + pageSize.value);
});

const selectedFiles = ref<Set<string>>(new Set());
const parsing = ref(false);
const batchUploading = ref(false);
const batchProgress = ref(0);

/** 请求目录访问权限并扫描音频文件 */
const requestDirectoryAccess = async () => {
  if (supportsFSAccess.value) {
    parsing.value = true;
    allScannedFiles.value = [];
    currentPage.value = 1;
    try {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const handle = await (window as any).showDirectoryPicker({ mode: "read" });
      directoryHandle.value = handle;
      await scanDirectory(handle);
      if (allScannedFiles.value.length >= fileScanLimit) {
        ElMessage.warning(`Scan stopped at ${fileScanLimit} files.`);
      } else if (allScannedFiles.value.length > 0) {
        ElMessage.success(`Found ${allScannedFiles.value.length} audio file(s).`);
      } else {
        ElMessage.info("No audio files found.");
      }
    } catch (error: unknown) {
      if (error instanceof DOMException && error.name !== "AbortError") {
        ElMessage.error(error.message || t("settings.local_access_error"));
      }
    } finally {
      parsing.value = false;
    }
  } else {
    directoryInputRef.value?.click();
  }
};

/** 处理目录选择器的文件变更事件 */
const handleDirectoryInputChange = async (event: Event) => {
  const input = event.target as HTMLInputElement;
  const files = input.files;
  if (!files || files.length === 0) return;

  parsing.value = true;
  allScannedFiles.value = [];
  currentPage.value = 1;

  for (const file of Array.from(files)) {
    if (allScannedFiles.value.length >= fileScanLimit) break;
    const name = file.name.toLowerCase();
    if (/\.(mp3|wav|flac|ogg|m4a|aac|wma|aiff|ape)$/.test(name)) {
      allScannedFiles.value.push({
        file,
        name: file.name,
        path: (file as File & { webkitRelativePath: string }).webkitRelativePath || file.name,
        size: file.size,
        type: file.type,
        metadata: null,
        loading: false,
      });
      await new Promise((resolve) => setTimeout(resolve, scanDelayMs));
    }
  }
  parsing.value = false;
  input.value = "";
};

/** 递归扫描目录中的音频文件 */
const scanDirectory = async (dirHandle: FileSystemDirectoryHandle, path = "") => {
  for await (const entry of dirHandle.values()) {
    if (allScannedFiles.value.length >= fileScanLimit) return;
    if (entry.kind === "file") {
      const name = entry.name.toLowerCase();
      if (/\.(mp3|wav|flac|ogg|m4a|aac|wma|aiff|ape)$/.test(name)) {
        const file = await entry.getFile();
        allScannedFiles.value.push({
          handle: entry,
          file,
          name: entry.name,
          path: path ? `${path}/${entry.name}` : entry.name,
          size: file.size,
          type: file.type,
          metadata: null,
          loading: false,
        });
        await new Promise((resolve) => setTimeout(resolve, scanDelayMs));
      }
    } else if (entry.kind === "directory") {
      await scanDirectory(entry, path ? `${path}/${entry.name}` : entry.name);
    }
  }
};

/** 切换文件选中状态 */
const toggleFileSelection = (path: string) => {
  if (selectedFiles.value.has(path)) {
    selectedFiles.value.delete(path);
  } else {
    selectedFiles.value.add(path);
  }
};

/** 解析单个文件的音频元数据 */
const parseFileMetadata = async (fileItem: ScannedFileItem) => {
  if (fileItem.metadata) return;
  fileItem.loading = true;
  try {
    fileItem.metadata = await parseAudioFile(fileItem.file);
  } catch (_e) {
    fileItem.metadata = { title: "", artist: "", album: "", year: "", track: "", genre: "", duration: "" };
  } finally {
    fileItem.loading = false;
  }
};

/** 批量解析选中文件的元数据 */
const parseAllSelectedMetadata = async () => {
  parsing.value = true;
  const selectedItems = paginatedFiles.value.filter((item: ScannedFileItem) => selectedFiles.value.has(item.path));
  for (const item of selectedItems) {
    if (!item.metadata) {
      await parseFileMetadata(item);
      await new Promise((resolve) => setTimeout(resolve, 50));
    }
  }
  parsing.value = false;
  ElMessage.success(`Parsed metadata for ${selectedItems.length} file(s).`);
};

/** 批量上传选中的文件 */
const uploadSelectedFiles = async () => {
  if (selectedFiles.value.size === 0) {
    ElMessage.warning("Please select files to upload.");
    return;
  }
  batchUploading.value = true;
  batchProgress.value = 0;
  const selectedItems = allScannedFiles.value.filter((item: ScannedFileItem) => selectedFiles.value.has(item.path));
  const total = selectedItems.length;
  let completed = 0;
  for (const item of selectedItems) {
    try {
      const metadata = item.metadata || {
        title: "",
        artist: "",
        album: "",
        year: "",
        track: "",
        genre: "",
        duration: "",
      };
      const createRes = await request.post<CreateMusicData>("/musics", {
        title: metadata.title || item.name.replace(/\.[^/.]+$/, ""),
        artist: metadata.artist || "",
        intro: "",
      });
      const musicId = createRes.data?.id;
      if (!musicId) {
        ElMessage.error(`Failed to create record for ${item.name}`);
        continue;
      }
      const fd = new FormData();
      fd.append("file", item.file);
      await request.post(`/musics/${musicId}/upload`, fd, {
        headers: { "Content-Type": "multipart/form-data" },
      });
      completed++;
      batchProgress.value = Math.round((completed / total) * 100);
    } catch (_e) {
      ElMessage.error(`Failed to upload ${item.name}`);
    }
  }
  batchUploading.value = false;
  if (completed > 0) ElMessage.success(`Uploaded ${completed} of ${total} file(s).`);
  selectedFiles.value.clear();
  batchProgress.value = 0;
};

/** 全选/取消全选 */
const selectAll = () => {
  const allSelected = paginatedFiles.value.every((item: ScannedFileItem) => selectedFiles.value.has(item.path));
  if (allSelected) {
    paginatedFiles.value.forEach((item: ScannedFileItem) => selectedFiles.value.delete(item.path));
  } else {
    paginatedFiles.value.forEach((item: ScannedFileItem) => selectedFiles.value.add(item.path));
  }
};

onMounted(() => {
  supportsFSAccess.value = !!(window as unknown as { showDirectoryPicker?: unknown }).showDirectoryPicker;
});
</script>

<template>
  <div class="batch-upload">
    <div class="batch-controls">
      <input
        ref="directoryInputRef"
        v-if="!supportsFSAccess"
        type="file"
        webkitdirectory
        multiple
        @change="handleDirectoryInputChange"
        style="display: none"
      />
      <el-button
        type="primary"
        :loading="parsing"
        @click="requestDirectoryAccess"
        :disabled="!!directoryHandle"
        size="large"
      >
        <el-icon style="margin-right: 6px"><FolderOpened /></el-icon>
        {{ $t("add.batch_import_desc") }}
      </el-button>

      <div class="batch-actions" v-if="allScannedFiles.length > 0">
        <el-button @click="selectAll">
          {{ selectedFiles.size === allScannedFiles.length ? $t("common.deselect_all") : $t("common.select_all") }}
        </el-button>
        <el-button
          type="primary"
          plain
          :loading="parsing"
          @click="parseAllSelectedMetadata"
          :disabled="selectedFiles.size === 0"
        >
          {{ $t("common.parse_metadata") }}
        </el-button>
        <el-button
          type="success"
          :loading="batchUploading"
          @click="uploadSelectedFiles"
          :disabled="selectedFiles.size === 0"
        >
          {{ $t("common.upload_count", { count: selectedFiles.size }) }}
        </el-button>
      </div>
    </div>

    <div v-if="batchProgress > 0" class="batch-progress-bar">
      <el-progress :percentage="batchProgress" :stroke-width="14" text-inside />
    </div>

    <div class="files-table" v-if="allScannedFiles.length > 0">
      <el-table :data="paginatedFiles" size="small">
        <el-table-column width="50">
          <template #default="{ row }">
            <el-checkbox :model-value="selectedFiles.has(row.path)" @change="toggleFileSelection(row.path)" />
          </template>
        </el-table-column>
        <el-table-column prop="name" :label="$t('add.file_name')" min-width="180" show-overflow-tooltip />
        <el-table-column :label="$t('add.size')" width="90">
          <template #default="{ row }">{{ formatFileSize(row.size) }}</template>
        </el-table-column>
        <el-table-column :label="$t('add.music_duration')" width="90">
          <template #default="{ row }">
            {{ row.metadata?.duration ? formatDuration(Number(row.metadata.duration)) : "—" }}
          </template>
        </el-table-column>
        <el-table-column :label="$t('add.music_artist')" width="140" show-overflow-tooltip>
          <template #default="{ row }">{{ row.metadata?.artist || "—" }}</template>
        </el-table-column>
        <el-table-column :label="$t('add.music_title')" min-width="160" show-overflow-tooltip>
          <template #default="{ row }">{{ row.metadata?.title || "—" }}</template>
        </el-table-column>
        <el-table-column width="80">
          <template #default="{ row }">
            <el-button v-if="!row.metadata" size="small" :loading="row.loading" @click="parseFileMetadata(row)">
              Parse
            </el-button>
          </template>
        </el-table-column>
      </el-table>
      <div class="table-footer">
        <span
          >{{ $t("common.total_files", { total: allScannedFiles.length }) }} ·
          {{ $t("common.selected_files", { count: selectedFiles.size }) }}</span
        >
        <el-pagination
          v-model:current-page="currentPage"
          :page-size="pageSize"
          :total="allScannedFiles.length"
          layout="prev, pager, next"
          background
          small
        />
      </div>
    </div>
    <div class="empty-state" v-else>
      <el-icon :size="48"><FolderOpened /></el-icon>
      <p>{{ $t("common.no_audio_found") }}</p>
      <p class="hint">{{ $t("common.start_import_hint") }}</p>
    </div>
  </div>
</template>

<style scoped lang="scss">
.batch-upload {
  padding: 4px 0;
}

.batch-controls {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  align-items: center;
  margin-bottom: 20px;
  padding-bottom: 16px;
  border-bottom: 1px solid var(--border-color);
}

.batch-actions {
  display: flex;
  gap: 8px;
  margin-left: auto;
}

.batch-progress-bar {
  margin-bottom: 16px;
}

.files-table {
  background: var(--bg-white);
  border-radius: 10px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.04);
  overflow: hidden;
}

.table-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 16px;
  border-top: 1px solid var(--border-color);
  font-size: 0.85rem;
  color: var(--text-light);
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 60px 20px;
  color: var(--text-light);
}

.empty-state p {
  margin: 8px 0 0;
}

.hint {
  font-size: 0.85rem;
  opacity: 0.6;
}
</style>
