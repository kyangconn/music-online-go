<script setup lang="ts">
import { FolderOpened } from "@element-plus/icons-vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { computed, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import BatchUploadResults from "@/components/upload/BatchUploadResults.vue";
import { useAudioPreprocessor } from "@/composables/useAudioPreprocessor";
import { useMusicDuplicates } from "@/composables/useMusicDuplicates";
import { useMusicUpload } from "@/composables/useMusicUpload";
import { useUploadPolicy } from "@/composables/useUploadPolicy";
import type { MusicMetadataFields, ScannedFileItem } from "@/types/api";
import type { BatchUploadResultItem } from "@/types/upload";
import {
  applyMetadataSuggestion,
  formatFileSize,
  isSupportedAudioFileName,
  validateUploadFile,
} from "@/utils/upload";

const { t } = useI18n();
const { preprocess } = useAudioPreprocessor();
const { checkDuplicate, enrichExactMatch } = useMusicDuplicates();
const { uploadOne } = useMusicUpload();
const { policy, loadPolicy } = useUploadPolicy();

const allScannedFiles = ref<ScannedFileItem[]>([]);
const currentPage = ref(1);
const pageSize = ref(10);
const fileScanLimit = 500;
const supportsFSAccess = ref(false);
const directoryInputRef = ref<HTMLInputElement>();

const paginatedFiles = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value;
  return allScannedFiles.value.slice(start, start + pageSize.value);
});

const selectedFiles = ref<Set<string>>(new Set());
const parsing = ref(false);
const preprocessProgress = ref(0);
const batchUploading = ref(false);
const batchProgress = ref(0);
const uploadResults = ref<BatchUploadResultItem[]>([]);
const defaultArtist = computed(() => t("add.unknown_artist"));

const emptyMetadata = (): MusicMetadataFields => ({
  title: "",
  artist: "",
  album: "",
  year: "",
  track: "",
  genre: "",
  duration: "",
});

const metadataForItem = (item: ScannedFileItem) => {
  if (!item.metadata) item.metadata = emptyMetadata();
  if (!item.metadata.title) item.metadata.title = item.name.replace(/\.[^/.]+$/, "");
  if (!item.metadata.artist) item.metadata.artist = defaultArtist.value;
  return item.metadata;
};

const findResult = (path: string) => uploadResults.value.find((item) => item.path === path);

const updateResult = (path: string, patch: Partial<BatchUploadResultItem>) => {
  const result = findResult(path);
  if (result) Object.assign(result, patch);
};

const updateBatchProgress = () => {
  if (!uploadResults.value.length) {
    batchProgress.value = 0;
    return;
  }
  const finished = uploadResults.value.filter((item) => ["success", "failed", "skipped"].includes(item.status)).length;
  batchProgress.value = Math.round((finished / uploadResults.value.length) * 100);
};

const clearBatchResults = () => {
  uploadResults.value = [];
  batchProgress.value = 0;
};

/** 请求目录访问权限并扫描音频文件 */
const requestDirectoryAccess = async () => {
  await loadPolicy();
  if (supportsFSAccess.value && window.showDirectoryPicker) {
    parsing.value = true;
    clearBatchResults();
    allScannedFiles.value = [];
    selectedFiles.value.clear();
    currentPage.value = 1;
    try {
      const handle = await window.showDirectoryPicker({ id: "music-import", mode: "read" });
      const skipped = await scanDirectory(handle);
      if (allScannedFiles.value.length >= fileScanLimit) {
        ElMessage.warning(t("add.scan_stopped", { count: fileScanLimit }));
      } else if (allScannedFiles.value.length > 0) {
        ElMessage.success(t("add.found_audio_files", { count: allScannedFiles.value.length }));
      } else {
        ElMessage.info(t("add.no_audio_files_found"));
      }
      if (skipped > 0) {
        ElMessage.warning(t("add.skipped_invalid_files", { count: skipped }));
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
  clearBatchResults();
  allScannedFiles.value = [];
  selectedFiles.value.clear();
  currentPage.value = 1;
  let skipped = 0;

  for (const file of Array.from(files)) {
    if (allScannedFiles.value.length >= fileScanLimit) break;
    if (isSupportedAudioFileName(file.name, policy.value)) {
      const validation = validateUploadFile(file, "audio", policy.value);
      if (!validation.valid) {
        skipped++;
        continue;
      }
      allScannedFiles.value.push({
        file,
        name: file.name,
        path: (file as File & { webkitRelativePath: string }).webkitRelativePath || file.name,
        size: file.size,
        type: file.type,
        metadata: null,
        loading: false,
      });
    }
  }
  parsing.value = false;
  input.value = "";
  if (skipped > 0) {
    ElMessage.warning(t("add.skipped_invalid_files", { count: skipped }));
  }
};

/** 递归扫描目录中的音频文件 */
const scanDirectory = async (dirHandle: FileSystemDirectoryHandle, path = ""): Promise<number> => {
  let skipped = 0;
  for await (const entry of dirHandle.values()) {
    if (allScannedFiles.value.length >= fileScanLimit) return skipped;
    if (entry.kind === "file") {
      if (isSupportedAudioFileName(entry.name, policy.value)) {
        const file = await entry.getFile();
        const validation = validateUploadFile(file, "audio", policy.value);
        if (!validation.valid) {
          skipped++;
          continue;
        }
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
      }
    } else if (entry.kind === "directory") {
      skipped += await scanDirectory(entry, path ? `${path}/${entry.name}` : entry.name);
    }
  }
  return skipped;
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
  if (fileItem.metadata && fileItem.hash) return;
  fileItem.loading = true;
  fileItem.processingError = false;
  try {
    const result = await preprocess(fileItem.file);
    fileItem.metadata = result.metadata;
    fileItem.hash = result.hash;
    const duplicate = allScannedFiles.value.find(
      (item) => item.path !== fileItem.path && item.hash === result.hash,
    );
    fileItem.duplicateOf = duplicate?.path;
  } catch {
    fileItem.processingError = true;
    fileItem.metadata = emptyMetadata();
  } finally {
    fileItem.loading = false;
  }
};

/** 批量解析选中文件的元数据 */
const parseAllSelectedMetadata = async () => {
  parsing.value = true;
  preprocessProgress.value = 0;
  const selectedItems = allScannedFiles.value.filter((item: ScannedFileItem) => selectedFiles.value.has(item.path));
  let processed = 0;
  for (const item of selectedItems) {
    if (!item.metadata || !item.hash) {
      await parseFileMetadata(item);
    }
    processed++;
    preprocessProgress.value = Math.round((processed / selectedItems.length) * 100);
  }
  parsing.value = false;
  const duplicates = selectedItems.filter((item) => item.duplicateOf).length;
  if (duplicates > 0) ElMessage.warning(t("add.duplicates_found", { count: duplicates }));
  ElMessage.success(t("add.parse_metadata_success", { count: selectedItems.length }));
  preprocessProgress.value = 0;
};

const preflightItem = async (item: ScannedFileItem) => {
  updateResult(item.path, { status: "uploading", stage: "preflight", reason: t("add.result_preflight") });
  if (!item.metadata || !item.hash) await parseFileMetadata(item);
  const metadata = metadataForItem(item);

  if (item.duplicateOf) {
    updateResult(item.path, {
      status: "skipped",
      stage: "preflight",
      reason: t("add.result_local_duplicate", { name: item.duplicateOf }),
    });
    updateBatchProgress();
    return false;
  }

  try {
    const duplicate = await checkDuplicate(metadata, item.hash);
    applyMetadataSuggestion(metadata, duplicate.suggested_metadata);
    item.exactMatch = duplicate.exact_match;
    item.metadataMatches = duplicate.metadata_matches;
    item.enrichment = duplicate.enrichment;

    if (duplicate.exact_match) {
      const enriched = await enrichExactMatch(duplicate);
      updateResult(item.path, {
        status: "skipped",
        stage: "preflight",
        musicId: duplicate.exact_match.id,
        reason: enriched ? t("add.result_duplicate_enriched") : t("add.result_server_duplicate"),
      });
      updateBatchProgress();
      return false;
    }

    updateResult(item.path, {
      status: "pending",
      stage: "upload",
      reason: duplicate.metadata_matches.length
        ? t("add.result_possible_duplicate", { count: duplicate.metadata_matches.length })
        : t("add.result_ready"),
    });
    return true;
  } catch {
    updateResult(item.path, {
      status: "failed",
      stage: "preflight",
      reason: t("add.duplicate_check_failed"),
    });
    updateBatchProgress();
    return false;
  }
};

const uploadPreparedItem = async (item: ScannedFileItem, existingMusicId?: number) => {
  const metadata = metadataForItem(item);
  const previous = findResult(item.path);
  updateResult(item.path, {
    status: "uploading",
    stage: "upload",
    reason: t("add.result_uploading"),
    attempts: (previous?.attempts ?? 0) + 1,
  });

  const result = await uploadOne({
    title: metadata.title,
    artist: metadata.artist,
    metadata,
    audio: item.file,
    existingMusicId,
    silent: true,
  });
  if (result.success) {
    updateResult(item.path, {
      status: "success",
      musicId: result.musicId,
      reason: t("add.result_uploaded"),
    });
  } else {
    updateResult(item.path, {
      status: "failed",
      stage: "upload",
      musicId: result.musicId,
      reason: result.errorMessage || t("add.upload_file_failed", { name: item.name }),
    });
  }
  updateBatchProgress();
};

const initializeResults = (items: ScannedFileItem[]) => {
  uploadResults.value = items.map((item) => ({
    path: item.path,
    name: item.name,
    status: "pending",
    stage: "preflight",
    reason: t("add.result_waiting"),
    attempts: 0,
  }));
};

/** 批量上传选中的文件 */
const uploadSelectedFiles = async () => {
  if (selectedFiles.value.size === 0) {
    ElMessage.warning(t("add.select_files_required"));
    return;
  }

  const selectedItems = allScannedFiles.value.filter((item: ScannedFileItem) => selectedFiles.value.has(item.path));
  initializeResults(selectedItems);
  batchUploading.value = true;
  batchProgress.value = 0;
  try {
    const readyItems: ScannedFileItem[] = [];
    for (const item of selectedItems) {
      if (await preflightItem(item)) readyItems.push(item);
    }

    const possibleDuplicates = readyItems.filter((item) => item.metadataMatches?.length);
    if (possibleDuplicates.length) {
      const confirmed = await ElMessageBox.confirm(
        t("add.batch_possible_duplicate_confirm", { count: possibleDuplicates.length }),
        t("add.possible_duplicate_title"),
        { type: "warning", confirmButtonText: t("add.upload_anyway"), cancelButtonText: t("common.cancel") },
      )
        .then(() => true)
        .catch(() => false);
      if (!confirmed) return;
    }

    for (const item of readyItems) await uploadPreparedItem(item);
    const completed = uploadResults.value.filter((item) => item.status === "success").length;
    const failedPaths = uploadResults.value.filter((item) => item.status === "failed").map((item) => item.path);
    selectedFiles.value = new Set(failedPaths);
    ElMessage.success(t("add.upload_completed", { completed, total: selectedItems.length }));
  } finally {
    batchUploading.value = false;
    updateBatchProgress();
  }
};

const retryResultInternal = async (path: string) => {
  const item = allScannedFiles.value.find((candidate) => candidate.path === path);
  const result = findResult(path);
  if (!item || !result || result.status !== "failed") return;

  const ready = result.stage === "preflight" ? await preflightItem(item) : true;
  if (ready) await uploadPreparedItem(item, result.musicId);
};

const retryResult = async (path: string) => {
  batchUploading.value = true;
  try {
    await retryResultInternal(path);
  } finally {
    batchUploading.value = false;
    updateBatchProgress();
  }
};

const retryAllFailed = async () => {
  batchUploading.value = true;
  try {
    const failedPaths = uploadResults.value.filter((item) => item.status === "failed").map((item) => item.path);
    for (const path of failedPaths) await retryResultInternal(path);
  } finally {
    batchUploading.value = false;
    updateBatchProgress();
  }
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
  supportsFSAccess.value = typeof window.showDirectoryPicker === "function";
  void loadPolicy();
});
</script>

<template>
  <div class="batch-upload">
    <div class="batch-controls">
      <input
        ref="directoryInputRef"
        class="directory-input"
        type="file"
        webkitdirectory
        multiple
        @change="handleDirectoryInputChange"
      />
      <el-button
        type="primary"
        :loading="parsing"
        :disabled="batchUploading"
        @click="requestDirectoryAccess"
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
          :disabled="selectedFiles.size === 0 || batchUploading"
        >
          {{ $t("common.parse_metadata") }}
        </el-button>
        <el-button
          type="success"
          :loading="batchUploading"
          @click="uploadSelectedFiles"
          :disabled="selectedFiles.size === 0 || parsing"
        >
          {{ $t("common.upload_count", { count: selectedFiles.size }) }}
        </el-button>
      </div>
    </div>

    <div v-if="batchProgress > 0" class="batch-progress-bar">
      <el-progress :percentage="batchProgress" :stroke-width="14" text-inside />
    </div>

    <div v-if="preprocessProgress > 0" class="batch-progress-bar">
      <el-progress :percentage="preprocessProgress" :stroke-width="14" :format="() => $t('add.preprocessing')" />
    </div>

    <BatchUploadResults
      :results="uploadResults"
      :uploading="batchUploading"
      @retry="retryResult"
      @retry-all="retryAllFailed"
      @clear="clearBatchResults"
    />

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
            {{ row.metadata?.duration || "—" }}
          </template>
        </el-table-column>
        <el-table-column :label="$t('add.music_artist')" width="140" show-overflow-tooltip>
          <template #default="{ row }">{{ row.metadata?.artist || "—" }}</template>
        </el-table-column>
        <el-table-column :label="$t('add.music_title')" min-width="160" show-overflow-tooltip>
          <template #default="{ row }">{{ row.metadata?.title || "—" }}</template>
        </el-table-column>
        <el-table-column :label="$t('add.preprocess_status')" min-width="150" show-overflow-tooltip>
          <template #default="{ row }">
            <el-tag v-if="row.duplicateOf" type="warning" size="small">
              {{ $t("add.duplicate_of", { name: row.duplicateOf }) }}
            </el-tag>
            <el-tag v-else-if="row.exactMatch" type="warning" size="small">
              {{ $t("add.exact_duplicate_existing") }}
            </el-tag>
            <el-tag v-else-if="row.metadataMatches?.length" type="warning" size="small">
              {{ $t("add.possible_duplicate") }}
            </el-tag>
            <el-tag v-else-if="row.processingError" type="danger" size="small">
              {{ $t("add.preprocess_failed") }}
            </el-tag>
            <el-tag v-else-if="row.hash" type="success" size="small">
              {{ $t("add.preprocessed") }}
            </el-tag>
            <span v-else>—</span>
          </template>
        </el-table-column>
        <el-table-column width="80">
          <template #default="{ row }">
            <el-button
              v-if="!row.metadata || row.processingError"
              size="small"
              :loading="row.loading"
              @click="parseFileMetadata(row)"
            >
              {{ $t("add.parse") }}
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

.directory-input {
  display: none;
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
  border-radius: 8px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.04);
  overflow: hidden;
}

.batch-upload :deep(.files-table:has(.el-tag--warning)) {
  box-shadow:
    inset 3px 0 0 var(--el-color-warning),
    0 2px 8px rgba(0, 0, 0, 0.04);
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
