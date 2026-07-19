<script setup lang="ts">
import { CircleClose, Delete, Edit, FolderAdd, Refresh, VideoPlay, View } from "@element-plus/icons-vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { computed, onBeforeUnmount, onMounted, reactive, ref } from "vue";
import { useI18n } from "vue-i18n";
import type {
  MediaLibraryRoot,
  MediaScanJob,
  MediaScanJobDetail,
  MediaScanStatus,
  PaginatedData,
} from "@/types/api";
import { useApiError } from "@/composables/useApiError";
import request from "@/utils/request";

const { t, locale } = useI18n();
const { handleError } = useApiError();
const roots = ref<MediaLibraryRoot[]>([]);
const jobs = ref<MediaScanJob[]>([]);
const rootsLoading = ref(false);
const jobsLoading = ref(false);
const saving = ref(false);
const probingRootID = ref<number | null>(null);
const rootDialogVisible = ref(false);
const detailsVisible = ref(false);
const selectedJob = ref<MediaScanJobDetail | null>(null);
const editingRootID = ref<number | null>(null);
const rootForm = reactive({
  name: "",
  path: "",
  enabled: true,
  storageKind: "auto",
  expectedFilesystem: "",
  probeFile: "",
  pathSemantics: "auto",
});
let pollTimer: ReturnType<typeof setInterval> | undefined;

const hasActiveJobs = computed(() =>
  jobs.value.some((job) => job.status === "pending" || job.status === "running" || job.status === "retry_wait"),
);

const fetchRoots = async (silent = false) => {
	if (!silent) rootsLoading.value = true;
  try {
    const response = await request.get<MediaLibraryRoot[]>("/users/admin/media-library/roots");
    roots.value = response.data;
  } catch (error) {
		if (!silent) handleError(error, t("admin.media_library_load_failed"));
	} finally {
		if (!silent) rootsLoading.value = false;
  }
};

const fetchJobs = async (silent = false) => {
  if (!silent) jobsLoading.value = true;
  try {
    const response = await request.get<PaginatedData<MediaScanJob>>("/users/admin/media-library/scans", {
      params: { page: 1, page_size: 20 },
    });
    jobs.value = response.data.items;
  } catch (error) {
    if (!silent) handleError(error, t("admin.media_scan_load_failed"));
  } finally {
    if (!silent) jobsLoading.value = false;
  }
};

const refresh = () => Promise.all([fetchRoots(), fetchJobs()]);

const openCreate = () => {
  editingRootID.value = null;
  rootForm.name = "";
  rootForm.path = "";
  rootForm.enabled = true;
  rootForm.storageKind = "auto";
  rootForm.expectedFilesystem = "";
  rootForm.probeFile = "";
  rootForm.pathSemantics = "auto";
  rootDialogVisible.value = true;
};

const openEdit = (root: MediaLibraryRoot) => {
  editingRootID.value = root.id;
  rootForm.name = root.name;
  rootForm.path = root.path;
  rootForm.enabled = root.enabled;
  rootForm.storageKind = root.storage_kind;
  rootForm.expectedFilesystem = root.expected_filesystem;
  rootForm.probeFile = root.probe_file;
  rootForm.pathSemantics = root.path_semantics;
  rootDialogVisible.value = true;
};

const saveRoot = async () => {
  if (!rootForm.name.trim() || !rootForm.path.trim()) {
    ElMessage.warning(t("admin.media_root_required"));
    return;
  }
  saving.value = true;
  try {
    if (editingRootID.value === null) {
      await request.post("/users/admin/media-library/roots", {
        name: rootForm.name.trim(),
        path: rootForm.path.trim(),
        storage_kind: rootForm.storageKind,
        expected_filesystem: rootForm.expectedFilesystem.trim(),
        probe_file: rootForm.probeFile.trim(),
        path_semantics: rootForm.pathSemantics,
      });
      ElMessage.success(t("admin.media_root_created"));
    } else {
      await request.patch(`/users/admin/media-library/roots/${editingRootID.value}`, {
        name: rootForm.name.trim(),
        path: rootForm.path.trim(),
        enabled: rootForm.enabled,
        storage_kind: rootForm.storageKind,
        expected_filesystem: rootForm.expectedFilesystem.trim(),
        probe_file: rootForm.probeFile.trim(),
        path_semantics: rootForm.pathSemantics,
      });
      ElMessage.success(t("admin.media_root_updated"));
    }
    rootDialogVisible.value = false;
    await fetchRoots();
  } catch (error) {
    handleError(error, t("admin.media_root_save_failed"));
  } finally {
    saving.value = false;
  }
};

const deleteRoot = async (root: MediaLibraryRoot) => {
  try {
    await ElMessageBox.confirm(t("admin.media_root_delete_confirm", { name: root.name }), t("admin.media_root_delete"), {
      confirmButtonText: t("common.delete"),
      cancelButtonText: t("common.cancel"),
      type: "warning",
    });
    await request.delete(`/users/admin/media-library/roots/${root.id}`);
    ElMessage.success(t("admin.media_root_deleted"));
    await fetchRoots();
  } catch (error) {
    if (error !== "cancel") handleError(error, t("admin.media_root_delete_failed"));
  }
};

const activeJobForRoot = (rootID: number) =>
  jobs.value.find(
    (job) => job.root_id === rootID &&
      (job.status === "pending" || job.status === "running" || job.status === "retry_wait"),
  );

const probeRoot = async (root: MediaLibraryRoot) => {
  probingRootID.value = root.id;
  try {
    await request.post(`/users/admin/media-library/roots/${root.id}/probe`);
    await fetchRoots();
  } catch (error) {
    handleError(error, t("admin.media_root_probe_failed"));
  } finally {
    probingRootID.value = null;
  }
};

const startScan = async (root: MediaLibraryRoot) => {
  try {
    await request.post(`/users/admin/media-library/roots/${root.id}/scans`);
    ElMessage.success(t("admin.media_scan_queued"));
    await fetchJobs();
  } catch (error) {
    handleError(error, t("admin.media_scan_start_failed"));
  }
};

const cancelScan = async (job: MediaScanJob) => {
  try {
    await request.post(`/users/admin/media-library/scans/${job.id}/cancel`);
    ElMessage.success(t("admin.media_scan_cancel_requested"));
    await fetchJobs();
  } catch (error) {
    handleError(error, t("admin.media_scan_cancel_failed"));
  }
};

const viewScan = async (job: MediaScanJob) => {
  try {
    const response = await request.get<MediaScanJobDetail>(`/users/admin/media-library/scans/${job.id}`);
    selectedJob.value = response.data;
    detailsVisible.value = true;
  } catch (error) {
    handleError(error, t("admin.media_scan_load_failed"));
  }
};

const statusType = (status: MediaScanStatus) => {
  if (status === "succeeded") return "success";
  if (status === "failed") return "danger";
  if (status === "running") return "primary";
  if (status === "cancelled") return "info";
  return "warning";
};

const healthType = (root: MediaLibraryRoot) => {
  if (!root.enabled || root.health.status === "unknown") return "info";
  if (root.health.status === "online") return "success";
  if (root.health.status === "offline") return "danger";
  return "warning";
};

const healthLabel = (root: MediaLibraryRoot) => {
  if (!root.enabled) return t("admin.disabled");
  if (root.storage_kind === "nfs" && root.health.status === "offline") return t("admin.media_root_nfs_offline");
  return t(`admin.media_root_health_${root.health.status}`);
};

const formatTime = (value?: string) => {
  if (!value) return "—";
  return new Intl.DateTimeFormat(locale.value, { dateStyle: "short", timeStyle: "medium" }).format(new Date(value));
};

onMounted(async () => {
  await refresh();
  pollTimer = setInterval(() => {
	if (hasActiveJobs.value) void Promise.all([fetchJobs(true), fetchRoots(true)]);
  }, 2500);
});

onBeforeUnmount(() => {
  if (pollTimer) clearInterval(pollTimer);
});
</script>

<template>
  <section class="media-library-panel">
    <el-alert :title="$t('admin.media_library_append_notice')" type="info" :closable="false" show-icon />

    <div class="section-heading">
      <div>
        <h3>{{ $t("admin.media_roots") }}</h3>
        <p>{{ $t("admin.media_roots_help") }}</p>
      </div>
      <div class="actions">
        <el-button :icon="Refresh" @click="refresh">{{ $t("admin.refresh") }}</el-button>
        <el-button type="primary" :icon="FolderAdd" @click="openCreate">{{ $t("admin.media_root_add") }}</el-button>
      </div>
    </div>

    <el-table v-loading="rootsLoading" :data="roots" row-key="id" size="small" class="admin-table">
      <el-table-column prop="name" :label="$t('admin.media_root_name')" min-width="150">
        <template #default="{ row }">
          {{ row.kind === "managed" ? $t("admin.media_root_default_name") : row.name }}
        </template>
      </el-table-column>
      <el-table-column prop="path" :label="$t('admin.media_root_path')" min-width="260" show-overflow-tooltip>
        <template #default="{ row }"><code>{{ row.path }}</code></template>
      </el-table-column>
      <el-table-column :label="$t('admin.media_root_kind')" width="130">
        <template #default="{ row }">
          <el-tag :type="row.kind === 'managed' ? 'success' : 'info'">
            {{ $t(`admin.media_root_${row.kind}`) }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column :label="$t('admin.media_root_storage_kind')" width="120">
        <template #default="{ row }">
          {{ $t(`admin.media_storage_${row.storage_kind}`) }}
          <span v-if="row.health.filesystem" class="filesystem">({{ row.health.filesystem }})</span>
        </template>
      </el-table-column>
      <el-table-column :label="$t('admin.media_root_status')" min-width="260">
        <template #default="{ row }">
          <el-tooltip
            :content="`${row.health.code}: ${row.health.message}`"
            placement="top"
            :disabled="!row.health.message"
          >
            <el-tag :type="healthType(row)">{{ healthLabel(row) }}</el-tag>
          </el-tooltip>
          <div v-if="row.health.last_checked_at" class="health-time">{{ formatTime(row.health.last_checked_at) }}</div>
		  <div
			v-if="row.health.message && row.health.status !== 'online'"
			class="health-reason"
		  >
			{{ row.health.code }} · {{ row.health.message }}
		  </div>
        </template>
      </el-table-column>
      <el-table-column :label="$t('admin.actions')" width="230" fixed="right">
        <template #default="{ row }">
          <el-button
            :icon="Refresh"
            circle
            size="small"
            :loading="probingRootID === row.id"
            :title="$t('admin.media_root_probe')"
            @click="probeRoot(row)"
          />
          <el-button
            :icon="VideoPlay"
            circle
            size="small"
            type="primary"
            :title="$t('admin.media_scan_start')"
            :disabled="!row.enabled || Boolean(activeJobForRoot(row.id))"
            @click="startScan(row)"
          />
          <el-button
            v-if="row.kind !== 'managed'"
            :icon="Edit"
            circle
            size="small"
            :title="$t('common.edit')"
            @click="openEdit(row)"
          />
          <el-button
            v-if="row.kind !== 'managed'"
            :icon="Delete"
            circle
            size="small"
            type="danger"
            :title="$t('common.delete')"
            @click="deleteRoot(row)"
          />
        </template>
      </el-table-column>
    </el-table>

    <div class="section-heading scans-heading">
      <div>
        <h3>{{ $t("admin.media_scan_history") }}</h3>
        <p>{{ $t("admin.media_scan_history_help") }}</p>
      </div>
    </div>

    <el-table v-loading="jobsLoading" :data="jobs" row-key="id" size="small" class="admin-table">
      <el-table-column prop="id" label="#" width="70" />
      <el-table-column prop="root_name" :label="$t('admin.media_root_name')" min-width="140">
        <template #default="{ row }">
          {{ row.root_id === 0 ? $t("admin.media_root_default_name") : row.root_name }}
        </template>
      </el-table-column>
      <el-table-column :label="$t('admin.media_scan_status')" width="120">
        <template #default="{ row }">
          <el-tag :type="statusType(row.status)">{{ $t(`admin.media_scan_${row.status}`) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column :label="$t('admin.media_scan_progress')" min-width="210">
        <template #default="{ row }">
          {{ row.processed_count }} / {{ row.discovered_count }} ·
          {{ $t("admin.media_scan_imported", { count: row.imported_count }) }}
        </template>
      </el-table-column>
      <el-table-column :label="$t('admin.media_scan_issues')" width="120">
        <template #default="{ row }">{{ row.warning_count }} / {{ row.failed_count }}</template>
      </el-table-column>
      <el-table-column :label="$t('admin.media_scan_started')" min-width="170">
        <template #default="{ row }">{{ formatTime(row.started_at || row.created_at) }}</template>
      </el-table-column>
      <el-table-column :label="$t('admin.actions')" width="120" fixed="right">
        <template #default="{ row }">
          <el-button :icon="View" circle size="small" @click="viewScan(row)" />
          <el-button
            v-if="row.status === 'pending' || row.status === 'running' || row.status === 'retry_wait'"
            :icon="CircleClose"
            circle
            size="small"
            type="warning"
            :disabled="row.cancel_requested"
            @click="cancelScan(row)"
          />
        </template>
      </el-table-column>
    </el-table>

    <el-dialog
      v-model="rootDialogVisible"
      :title="editingRootID === null ? $t('admin.media_root_add') : $t('admin.media_root_edit')"
      width="min(520px, 92vw)"
    >
      <el-form label-position="top">
        <el-form-item :label="$t('admin.media_root_name')" required>
          <el-input v-model="rootForm.name" maxlength="100" />
        </el-form-item>
        <el-form-item :label="$t('admin.media_root_path')" required>
          <el-input v-model="rootForm.path" :placeholder="$t('admin.media_root_path_placeholder')" />
          <div class="form-help">{{ $t("admin.media_root_path_help") }}</div>
        </el-form-item>
        <el-form-item :label="$t('admin.media_root_storage_kind')">
          <el-select v-model="rootForm.storageKind" class="full-width">
            <el-option :label="$t('admin.media_storage_auto')" value="auto" />
            <el-option :label="$t('admin.media_storage_local')" value="local" />
            <el-option :label="$t('admin.media_storage_nfs')" value="nfs" />
            <el-option :label="$t('admin.media_storage_smb')" value="smb" />
          </el-select>
          <div class="form-help">{{ $t("admin.media_root_storage_kind_help") }}</div>
        </el-form-item>
        <el-form-item :label="$t('admin.media_root_expected_filesystem')">
          <el-input v-model="rootForm.expectedFilesystem" placeholder="nfs4 / cifs / ext4" maxlength="64" />
          <div class="form-help">{{ $t("admin.media_root_expected_filesystem_help") }}</div>
        </el-form-item>
        <el-form-item :label="$t('admin.media_root_probe_file')">
          <el-input v-model="rootForm.probeFile" placeholder=".music-online-probe" maxlength="500" />
          <div class="form-help">{{ $t("admin.media_root_probe_file_help") }}</div>
        </el-form-item>
        <el-form-item :label="$t('admin.media_root_path_semantics')">
          <el-select v-model="rootForm.pathSemantics" class="full-width">
            <el-option :label="$t('admin.media_path_auto')" value="auto" />
            <el-option :label="$t('admin.media_path_case_sensitive')" value="case_sensitive" />
            <el-option :label="$t('admin.media_path_case_insensitive')" value="case_insensitive" />
          </el-select>
          <div class="form-help">{{ $t("admin.media_root_path_semantics_help") }}</div>
        </el-form-item>
        <el-form-item v-if="editingRootID !== null" :label="$t('admin.media_root_status')">
          <el-switch v-model="rootForm.enabled" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="rootDialogVisible = false">{{ $t("common.cancel") }}</el-button>
        <el-button type="primary" :loading="saving" @click="saveRoot">{{ $t("common.save") }}</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="detailsVisible" :title="$t('admin.media_scan_details')" width="min(860px, 94vw)">
      <template v-if="selectedJob">
        <el-descriptions :column="2" border size="small">
          <el-descriptions-item :label="$t('admin.media_root_name')">
            {{ selectedJob.root_id === 0 ? $t("admin.media_root_default_name") : selectedJob.root_name }}
          </el-descriptions-item>
          <el-descriptions-item :label="$t('admin.media_scan_status')">
            {{ $t(`admin.media_scan_${selectedJob.status}`) }}
          </el-descriptions-item>
          <el-descriptions-item :label="$t('admin.media_scan_discovered')">{{ selectedJob.discovered_count }}</el-descriptions-item>
          <el-descriptions-item :label="$t('admin.media_scan_import_count')">{{ selectedJob.imported_count }}</el-descriptions-item>
          <el-descriptions-item :label="$t('admin.media_scan_existing')">{{ selectedJob.existing_count }}</el-descriptions-item>
          <el-descriptions-item :label="$t('admin.media_scan_duplicates')">{{ selectedJob.duplicate_count }}</el-descriptions-item>
          <el-descriptions-item :label="$t('admin.media_scan_attempt')">{{ selectedJob.attempt }}</el-descriptions-item>
		  <el-descriptions-item v-if="selectedJob.failure_code" :label="$t('admin.media_scan_failure_reason')">
			{{ selectedJob.failure_code }} ·
			{{ $t(selectedJob.failure_retryable ? 'admin.media_scan_retryable' : 'admin.media_scan_not_retryable') }}
		  </el-descriptions-item>
          <el-descriptions-item v-if="selectedJob.next_attempt_at" :label="$t('admin.media_scan_next_attempt')">
            {{ formatTime(selectedJob.next_attempt_at) }}
          </el-descriptions-item>
        </el-descriptions>
        <p v-if="selectedJob.error_summary" class="scan-summary">{{ selectedJob.error_summary }}</p>
        <el-table :data="selectedJob.issues" size="small" max-height="360" class="issues-table">
          <el-table-column prop="severity" :label="$t('admin.media_scan_issue_severity')" width="100" />
          <el-table-column prop="relative_path" :label="$t('admin.media_scan_issue_file')" min-width="220" show-overflow-tooltip />
          <el-table-column prop="message" :label="$t('admin.media_scan_issue_message')" min-width="280" />
        </el-table>
      </template>
    </el-dialog>
  </section>
</template>

<style scoped lang="scss">
.media-library-panel {
  width: 100%;
}

.section-heading {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: $spacing-lg;
  margin: $spacing-xl 0 $spacing-md;

  h3 {
    margin: 0 0 $spacing-xs;
    font-size: $fs-lg;
  }

  p {
    margin: 0;
    color: var(--text-secondary);
    font-size: $fs-sm;
  }
}

.scans-heading {
  margin-top: $spacing-2xl;
}

.actions {
  display: flex;
  flex-wrap: wrap;
  gap: $spacing-sm;
}

.admin-table {
  width: 100%;
}

code {
  color: var(--text-primary);
  word-break: break-all;
}

.form-help,
.scan-summary {
  color: var(--text-secondary);
  font-size: $fs-sm;
}

.full-width {
  width: 100%;
}

.filesystem,
.health-time,
.health-reason {
  color: var(--text-secondary);
  font-size: $fs-xs;
}

.filesystem {
  display: block;
}

.health-time {
  margin-top: $spacing-xs;
}

.health-reason {
	margin-top: $spacing-xs;
	word-break: break-word;
}

.scan-summary {
  margin: $spacing-md 0 0;
}

.issues-table {
  margin-top: $spacing-lg;
}

@media (max-width: 640px) {
  .section-heading {
    align-items: stretch;
    flex-direction: column;
  }
}
</style>
