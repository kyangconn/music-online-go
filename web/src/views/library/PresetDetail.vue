<script setup lang="ts">
import { List, VideoPlay } from "@element-plus/icons-vue";
import { ElMessage } from "element-plus";
import { computed, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { useRoute, useRouter } from "vue-router";
import InfoErrorState from "@/components/common/InfoErrorState.vue";
import MusicTrackTable from "@/components/music/MusicTrackTable.vue";
import { usePlayerStore } from "@/store/player";
import type { Music, PresetID, PresetSummary } from "@/types/api";
import { fetchMusicCollection } from "@/utils/library";
import { isPresetID } from "@/utils/presets";
import request from "@/utils/request";

const route = useRoute();
const router = useRouter();
const { t } = useI18n();
const playerStore = usePlayerStore();
const tracks = ref<Music[]>([]);
const summary = ref<PresetSummary>();
const loading = ref(false);
const loadFailed = ref(false);
const preset = computed<PresetID | undefined>(() =>
  isPresetID(route.params.preset) ? route.params.preset : undefined,
);
const playableTracks = computed(() => tracks.value.filter((track) => Boolean(track.path)));
let loadVersion = 0;

const load = async () => {
  const currentPreset = preset.value;
  if (!currentPreset) {
    void router.replace({ name: "Presets" });
    return;
  }
  const version = ++loadVersion;
  tracks.value = [];
  summary.value = undefined;
  loading.value = true;
  loadFailed.value = false;
  try {
    const [presetResponse, music] = await Promise.all([
      request.get<{ items: PresetSummary[] }>("/presets"),
      fetchMusicCollection({ preset: currentPreset }),
    ]);
    if (version !== loadVersion) return;
    summary.value = presetResponse.data.items.find((item) => item.preset_id === currentPreset);
    tracks.value = music;
  } catch {
    if (version === loadVersion) loadFailed.value = true;
  } finally {
    if (version === loadVersion) loading.value = false;
  }
};

watch(preset, () => void load(), { immediate: true });

const playPreset = async () => {
  if (!(await playerStore.playCollection(tracks.value))) {
    ElMessage.info(t("library.no_playable_tracks"));
  }
};

const enqueuePreset = () => {
  const count = playerStore.enqueueTracks(tracks.value);
  if (count) ElMessage.success(t("library.queued_tracks", { count }));
  else ElMessage.info(playableTracks.value.length ? t("library.already_queued") : t("library.no_playable_tracks"));
};
</script>

<template>
  <section v-if="preset" class="page-section preset-detail">
    <el-skeleton v-if="loading && tracks.length === 0" :rows="7" animated />
    <InfoErrorState
      v-else-if="loadFailed"
      :description="$t('classification.preset_load_failed')"
      @retry="load"
    />
    <template v-else>
      <header :class="['preset-hero', `preset-hero--${preset}`]">
        <div class="preset-symbol" aria-hidden="true">♫</div>
        <div>
          <el-text type="info">{{ $t("classification.title") }}</el-text>
          <h1>{{ $t(`classification.${preset}`) }}</h1>
          <p>{{ $t(`classification.${preset}_desc`) }}</p>
          <div class="preset-meta">
            <span>{{ $t("library.track_count", { count: summary?.track_count || tracks.length }) }}</span>
          </div>
          <div class="preset-actions">
            <el-button type="primary" :icon="VideoPlay" :disabled="playableTracks.length === 0" @click="playPreset">
              {{ $t("classification.play_preset") }}
            </el-button>
            <el-button :icon="List" :disabled="playableTracks.length === 0" @click="enqueuePreset">
              {{ $t("library.add_to_queue") }}
            </el-button>
          </div>
        </div>
      </header>

      <el-alert
        v-if="(summary?.track_count || 0) > tracks.length"
        type="warning"
        :closable="false"
        :title="$t('library.collection_truncated', { count: tracks.length })"
      />
      <el-empty v-if="tracks.length === 0" :description="$t('classification.no_preset_tracks')" />
      <MusicTrackTable v-else :tracks="tracks" :playback-context="tracks" show-album />
    </template>
  </section>
</template>

<style scoped lang="scss">
.preset-hero {
  --preset-accent: #7467d9;

  display: grid;
  grid-template-columns: auto 1fr;
  align-items: start;
  gap: $spacing-xl;
  margin-bottom: $spacing-2xl;
  padding: $spacing-2xl;
  border-radius: $radius-xl;
  background: linear-gradient(135deg, color-mix(in srgb, var(--preset-accent) 18%, transparent), transparent);
  border-left: 5px solid var(--preset-accent);

  h1 {
    margin: $spacing-xs 0 $spacing-sm;
    font-size: clamp(2rem, 6vw, 4rem);
  }

  p {
    max-width: 760px;
    margin: 0;
    color: var(--text-secondary);
  }
}

.preset-hero--calm_flow {
  --preset-accent: #3aa981;
}

.preset-hero--kinetic_pulse {
  --preset-accent: #e69a2e;
}

.preset-hero--bass_impact {
  --preset-accent: #d95858;
}

.preset-symbol {
  display: grid;
  width: 72px;
  height: 72px;
  border-radius: $radius-xl;
  background: color-mix(in srgb, var(--preset-accent) 22%, transparent);
  color: var(--preset-accent);
  font-size: $fs-2xl;
  place-items: center;
}

.preset-meta,
.preset-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: $spacing-sm;
  margin-top: $spacing-md;
}

.preset-meta {
  color: var(--text-secondary);
}

@include mobile {
  .preset-hero {
    grid-template-columns: 1fr;
    padding: $spacing-lg;
  }
}
</style>
