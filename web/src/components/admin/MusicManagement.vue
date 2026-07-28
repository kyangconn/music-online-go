<script setup lang="ts">
import { DataAnalysis, Delete, Document, Refresh, Search, View } from "@element-plus/icons-vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { computed, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { useRouter } from "vue-router";
import type {
  AnalysisBackfillResponse,
  AnalysisScheduleResponse,
  AnalysisStatus,
  Music,
  PresetClassification,
  PresetID,
  PresetStatus,
} from "@/types/api";
import MusicCover from "@/components/music/MusicCover.vue";
import { useApiError } from "@/composables/useApiError";
import { usePaginatedFetch } from "@/composables/usePaginatedFetch";
import { useInstanceStore } from "@/store/instance";
import request from "@/utils/request";

const router = useRouter();
const { t } = useI18n();
const { handleError } = useApiError();
const instanceStore = useInstanceStore();
const query = ref("");
const presetFilter = ref<PresetID | "">("");
const statusFilter = ref<PresetStatus | "">("");
const busyMusicID = ref<number>();
const analysisMusicID = ref<number>();
const backfillBusy = ref(false);
const evidenceMusic = ref<Music>();
const presetIDs: PresetID[] = ["calm_flow", "kinetic_pulse", "cosmic_drift", "bass_impact"];
const presetStatuses: PresetStatus[] = ["classified", "needs_review", "unclassified"];

const { items: musics, loading, total, currentPage, pageSize, fetch: fetchMusics, resetAndFetch } =
  usePaginatedFetch<Music>("/musics", {
    errorMessageKey: "admin.load_music_failed",
    extraParams: computed(() => ({
      q: query.value || undefined,
      preset: presetFilter.value || undefined,
      preset_status: statusFilter.value || undefined,
    })),
  });

const handleSearch = () => {
  resetAndFetch();
};

const deleteMusic = async (music: Music) => {
  try {
    await ElMessageBox.confirm(
      t("admin.delete_music_confirm", { title: music.title }),
      t("admin.delete_music"),
      {
        confirmButtonText: t("common.delete"),
        cancelButtonText: t("common.cancel"),
        type: "warning",
      },
    );
    await request.delete(`/users/admin/musics/${music.id}`);
    ElMessage.success(t("admin.music_deleted"));
    fetchMusics();
  } catch (error) {
    if (error !== "cancel") handleError(error, t("admin.music_delete_failed"));
  }
};

const presetName = (preset: PresetID | "" | undefined) =>
  preset ? t(`classification.${preset}`) : t("classification.unclassified");

const presetTagType = (preset: PresetID | "" | undefined) => {
  if (preset === "calm_flow") return "success";
  if (preset === "kinetic_pulse") return "warning";
  if (preset === "bass_impact") return "danger";
  return "info";
};

const statusTagType = (status: PresetStatus | undefined) => {
  if (status === "classified") return "success";
  if (status === "needs_review") return "warning";
  return "info";
};

const replaceClassification = (music: Music, classification: PresetClassification) => {
  music.preset_classification = classification;
};

const updateManualPreset = async (music: Music, preset: PresetID | "") => {
  busyMusicID.value = music.id;
  try {
    const response = preset
      ? await request.put<PresetClassification>(`/users/admin/musics/${music.id}/classification/manual`, { preset })
      : await request.delete<PresetClassification>(`/users/admin/musics/${music.id}/classification/manual`);
    replaceClassification(music, response.data);
    ElMessage.success(t(preset ? "classification.override_saved" : "classification.override_cleared"));
  } catch (error) {
    handleError(error, t(preset ? "classification.override_failed" : "classification.clear_failed"));
  } finally {
    busyMusicID.value = undefined;
  }
};

const reclassify = async (music: Music) => {
  busyMusicID.value = music.id;
  try {
    const response = await request.post<PresetClassification>(
      `/users/admin/musics/${music.id}/classification/reclassify`,
    );
    replaceClassification(music, response.data);
    ElMessage.success(t("classification.reclassified"));
  } catch (error) {
    handleError(error, t("classification.reclassify_failed"));
  } finally {
    busyMusicID.value = undefined;
  }
};

const analysisStatusTagType = (status: AnalysisStatus | undefined) => {
  if (status === "succeeded") return "success";
  if (status === "pending" || status === "running") return "warning";
  if (status === "failed") return "danger";
  return "info";
};

const scheduleAnalysis = async (music: Music) => {
  analysisMusicID.value = music.id;
  try {
    const response = await request.post<AnalysisScheduleResponse>(`/users/admin/musics/${music.id}/analysis`, {
      include_audio: instanceStore.capabilities.audio_analyzer_enabled,
      force: true,
    });
    const job = response.data.audio_job;
    if (job) {
      music.audio_analysis = {
        job_id: job.id,
        status: job.status,
        analyzer_id: job.analyzer_id,
        analyzer_version: job.analyzer_version,
        model_version: job.model_version,
        attempt: job.attempt,
        error_code: job.error_code,
        completed_at: job.finished_at,
      };
    }
    ElMessage.success(t("classification.analysis_queued"));
  } catch (error) {
    handleError(error, t("classification.analysis_queue_failed"));
  } finally {
    analysisMusicID.value = undefined;
  }
};

const backfillAnalysis = async (includeAudio: boolean) => {
  try {
    await ElMessageBox.confirm(
      t(includeAudio ? "classification.backfill_audio_confirm" : "classification.backfill_rules_confirm"),
      t("classification.backfill"),
      {
        confirmButtonText: t("common.confirm"),
        cancelButtonText: t("common.cancel"),
        type: "info",
      },
    );
    backfillBusy.value = true;
    const response = await request.post<AnalysisBackfillResponse>("/users/admin/analysis/backfill", {
      include_audio: includeAudio,
    });
    ElMessage.success(
      t("classification.backfill_queued", {
        rules: response.data.rules_queued,
        audio: response.data.audio_queued,
        reused: response.data.reused,
      }),
    );
    await fetchMusics();
  } catch (error) {
    if (error !== "cancel") handleError(error, t("classification.backfill_failed"));
  } finally {
    backfillBusy.value = false;
  }
};

const evidenceLabel = (source: string, key: string) =>
  source === "genre"
    ? t("classification.evidence_genre", { key })
    : t("classification.evidence_other", { source, key });

onMounted(async () => {
  await instanceStore.load();
  await fetchMusics();
});
</script>

<template>
  <section class="admin-panel">
    <div class="admin-toolbar">
      <el-input
        v-model="query"
        :placeholder="$t('admin.search_music')"
        clearable
        class="search-input"
        @keyup.enter="handleSearch"
        @clear="handleSearch"
      >
        <template #prefix>
          <el-icon><Search /></el-icon>
        </template>
      </el-input>
      <el-button type="primary" :icon="Search" @click="handleSearch">{{ $t("admin.search") }}</el-button>
      <el-select
        v-model="presetFilter"
        clearable
        class="classification-filter"
        :placeholder="$t('classification.filter_preset')"
        @change="handleSearch"
      >
        <el-option v-for="preset in presetIDs" :key="preset" :label="presetName(preset)" :value="preset" />
      </el-select>
      <el-button :icon="Refresh" :loading="backfillBusy" @click="backfillAnalysis(false)">
        {{ $t("classification.backfill_rules") }}
      </el-button>
      <el-button
        v-if="instanceStore.capabilities.audio_analyzer_enabled"
        :icon="DataAnalysis"
        :loading="backfillBusy"
        @click="backfillAnalysis(true)"
      >
        {{ $t("classification.backfill_audio") }}
      </el-button>
      <el-select
        v-model="statusFilter"
        clearable
        class="classification-filter"
        :placeholder="$t('classification.filter_status')"
        @change="handleSearch"
      >
        <el-option
          v-for="status in presetStatuses"
          :key="status"
          :label="$t(`classification.${status}`)"
          :value="status"
        />
      </el-select>
    </div>

    <el-table v-loading="loading" :data="musics" size="small" class="admin-table">
      <el-table-column width="72">
        <template #default="{ row }">
          <div class="cover-thumb">
            <MusicCover :src="row.img || row.cover_url" />
          </div>
        </template>
      </el-table-column>
      <el-table-column prop="title" :label="$t('add.music_title')" min-width="180" show-overflow-tooltip />
      <el-table-column prop="artist" :label="$t('add.music_artist')" min-width="160" show-overflow-tooltip />
      <el-table-column prop="user_id" :label="$t('admin.uploader_id')" width="100" />
      <el-table-column prop="like_count" :label="$t('common.likes')" width="90" />
      <el-table-column :label="$t('classification.title')" min-width="180">
        <template #default="{ row }">
          <el-tag :type="presetTagType(row.preset_classification?.effective_preset)" effect="plain">
            {{ presetName(row.preset_classification?.effective_preset) }}
          </el-tag>
          <span v-if="row.preset_classification" class="classification-source">
            {{ $t(`classification.${row.preset_classification.effective_source}`) }}
          </span>
        </template>
      </el-table-column>
      <el-table-column :label="$t('classification.filter_status')" width="130">
        <template #default="{ row }">
          <el-tag :type="statusTagType(row.preset_classification?.status)" size="small">
            {{ $t(`classification.${row.preset_classification?.status || 'unclassified'}`) }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column :label="$t('classification.confidence')" width="110">
        <template #default="{ row }">
          {{ row.preset_classification ? `${Math.round(row.preset_classification.confidence * 100)}%` : "—" }}
        </template>
      </el-table-column>
      <el-table-column :label="$t('classification.analysis_status')" width="140">
        <template #default="{ row }">
          <el-tooltip :content="row.audio_analysis?.error_code || ''" :disabled="!row.audio_analysis?.error_code">
            <el-tag :type="analysisStatusTagType(row.audio_analysis?.status)" size="small" effect="plain">
              {{ $t(`classification.analysis_${row.audio_analysis?.status || 'not_queued'}`) }}
            </el-tag>
          </el-tooltip>
        </template>
      </el-table-column>
      <el-table-column :label="$t('classification.set_manual')" min-width="190">
        <template #default="{ row }">
          <el-select
            :model-value="row.preset_classification?.manual_preset || ''"
            clearable
            :loading="busyMusicID === row.id"
            :placeholder="presetName(row.preset_classification?.automatic_preset)"
            @change="(value: PresetID | '') => updateManualPreset(row, value)"
          >
            <el-option v-for="preset in presetIDs" :key="preset" :label="presetName(preset)" :value="preset" />
          </el-select>
        </template>
      </el-table-column>
      <el-table-column :label="$t('admin.actions')" width="240" fixed="right">
        <template #default="{ row }">
          <el-button :icon="View" circle size="small" @click="router.push(`/music/${row.id}`)" />
          <el-tooltip :content="$t('classification.evidence')">
            <el-button :icon="Document" circle size="small" @click="evidenceMusic = row" />
          </el-tooltip>
          <el-tooltip :content="$t('classification.reclassify')">
            <el-button
              :icon="Refresh"
              circle
              size="small"
              :loading="busyMusicID === row.id"
              @click="reclassify(row)"
            />
          </el-tooltip>
          <el-tooltip :content="$t('classification.analyze')">
            <el-button
              :icon="DataAnalysis"
              circle
              size="small"
              :loading="analysisMusicID === row.id"
              @click="scheduleAnalysis(row)"
            />
          </el-tooltip>
          <el-button :icon="Delete" circle size="small" type="danger" @click="deleteMusic(row)" />
        </template>
      </el-table-column>
    </el-table>

    <div class="admin-pagination">
      <el-pagination
        v-model:current-page="currentPage"
        background
        layout="prev, pager, next"
        :page-size="pageSize"
        :total="total"
        @current-change="fetchMusics"
      />
    </div>

    <el-dialog
      :model-value="Boolean(evidenceMusic)"
      :title="$t('classification.evidence')"
      width="min(700px, 94vw)"
      @close="evidenceMusic = undefined"
    >
      <template v-if="evidenceMusic?.preset_classification">
        <el-descriptions :column="2" border>
          <el-descriptions-item :label="$t('classification.title')">
            {{ presetName(evidenceMusic.preset_classification.effective_preset) }}
          </el-descriptions-item>
          <el-descriptions-item :label="$t('classification.confidence')">
            {{ Math.round(evidenceMusic.preset_classification.confidence * 100) }}%
          </el-descriptions-item>
          <el-descriptions-item :label="$t('classification.filter_status')">
            {{ $t(`classification.${evidenceMusic.preset_classification.status}`) }}
          </el-descriptions-item>
          <el-descriptions-item :label="$t('classification.rule_version')">
            {{ evidenceMusic.preset_classification.rule_version }}
          </el-descriptions-item>
        </el-descriptions>
        <h3>{{ $t("classification.score_breakdown") }}</h3>
        <div
          v-for="score in evidenceMusic.preset_classification.scores"
          :key="score.preset_id"
          class="preset-score"
        >
          <div class="score-heading">
            <span>{{ presetName(score.preset_id) }}</span>
            <strong>{{ Math.round(score.score * 100) }}%</strong>
          </div>
          <el-progress :percentage="Math.round(score.score * 100)" :show-text="false" />
          <div v-if="score.evidence.length" class="evidence-list">
            <el-tag v-for="item in score.evidence" :key="`${item.source}:${item.key}`" size="small" effect="plain">
              {{ evidenceLabel(item.source, item.key) }} · {{ Math.round(item.weight * 100) }}%
            </el-tag>
          </div>
        </div>
        <el-alert
          v-if="evidenceMusic.preset_classification.evidence_summary.length === 0"
          :title="$t('classification.no_evidence')"
          type="info"
          :closable="false"
        />
        <el-alert
          v-for="summary in evidenceMusic.preset_classification.evidence_summary.filter((item) => !item.startsWith('genre:'))"
          :key="summary"
          :title="$t(`classification.${summary}`)"
          type="info"
          :closable="false"
        />
      </template>
      <el-empty v-else :description="$t('classification.no_evidence')" />
    </el-dialog>
  </section>
</template>

<style scoped lang="scss">
.admin-panel {
  width: 100%;
}

.admin-toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: $spacing-sm;
  margin-bottom: $spacing-lg;
}

.search-input {
  max-width: 320px;
}

.classification-filter {
  width: 190px;
}

.classification-source {
  margin-left: $spacing-xs;
  color: var(--text-secondary);
  font-size: $fs-xs;
}

.cover-thumb {
  width: 44px;
  aspect-ratio: 1;
  border-radius: $radius-sm;
  overflow: hidden;
}

.admin-table {
  width: 100%;
}

.admin-pagination {
  @include flex-center;
  margin-top: $spacing-lg;
}

.preset-score {
  margin-top: $spacing-md;
}

.score-heading {
  @include flex-between;
  margin-bottom: $spacing-xs;
}

.evidence-list {
  display: flex;
  flex-wrap: wrap;
  gap: $spacing-xs;
  margin-top: $spacing-xs;
}
</style>
