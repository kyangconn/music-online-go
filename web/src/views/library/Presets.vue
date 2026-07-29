<script setup lang="ts">
import { WarningFilled } from "@element-plus/icons-vue";
import { computed, onMounted, ref } from "vue";
import InfoErrorState from "@/components/common/InfoErrorState.vue";
import { useUserStore } from "@/store/user";
import type { PresetSummary } from "@/types/api";
import request from "@/utils/request";
import { presetIDs } from "@/utils/presets";

const userStore = useUserStore();
const summaries = ref<PresetSummary[]>([]);
const loading = ref(false);
const loadFailed = ref(false);
const summariesByID = computed(() => new Map(summaries.value.map((summary) => [summary.preset_id, summary])));

const load = async () => {
  loading.value = true;
  loadFailed.value = false;
  try {
    const response = await request.get<{ items: PresetSummary[] }>("/presets");
    summaries.value = response.data.items || [];
  } catch {
    loadFailed.value = true;
  } finally {
    loading.value = false;
  }
};

onMounted(() => void load());
</script>

<template>
  <section class="page-section preset-page">
    <header class="preset-heading">
      <div>
        <h1>{{ $t("classification.browse_title") }}</h1>
        <p>{{ $t("classification.browse_desc") }}</p>
      </div>
    </header>

    <div v-if="loading" class="loading-wrap"><el-skeleton :rows="5" animated /></div>
    <InfoErrorState
      v-else-if="loadFailed"
      :description="$t('classification.preset_load_failed')"
      @retry="load"
    />
    <div v-else class="preset-grid">
      <router-link
        v-for="preset in presetIDs"
        :key="preset"
        class="preset-link"
        :to="{ name: 'PresetDetail', params: { preset } }"
        :aria-label="$t(`classification.${preset}`)"
      >
        <el-card :class="['preset-card', `preset-card--${preset}`]" shadow="hover">
          <div class="preset-symbol" aria-hidden="true">♫</div>
          <div class="preset-copy">
            <h2>{{ $t(`classification.${preset}`) }}</h2>
            <p>{{ $t(`classification.${preset}_desc`) }}</p>
            <div class="preset-counts">
              <strong>
                {{ $t("library.track_count", { count: summariesByID.get(preset)?.track_count || 0 }) }}
              </strong>
              <el-tag
                v-if="userStore.isAdmin && (summariesByID.get(preset)?.needs_review_count || 0) > 0"
                type="warning"
                effect="plain"
                :icon="WarningFilled"
              >
                {{
                  $t("classification.review_count", {
                    count: summariesByID.get(preset)?.needs_review_count || 0,
                  })
                }}
              </el-tag>
            </div>
          </div>
        </el-card>
      </router-link>
    </div>
  </section>
</template>

<style scoped lang="scss">
.preset-heading {
  margin-bottom: $spacing-2xl;

  h1 {
    margin: 0 0 $spacing-xs;
  }

  p {
    max-width: 760px;
    margin: 0;
    color: var(--text-secondary);
  }
}

.preset-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: $spacing-xl;
}

.preset-link {
  color: inherit;
  text-decoration: none;
}

.preset-card {
  position: relative;
  height: 100%;
  overflow: hidden;
  border-top: 4px solid var(--preset-accent);

  &::after {
    position: absolute;
    right: -50px;
    bottom: -75px;
    width: 190px;
    height: 190px;
    border-radius: 50%;
    background: color-mix(in srgb, var(--preset-accent) 14%, transparent);
    content: "";
    pointer-events: none;
  }
}

.preset-card--calm_flow {
  --preset-accent: #3aa981;
}

.preset-card--kinetic_pulse {
  --preset-accent: #e69a2e;
}

.preset-card--cosmic_drift {
  --preset-accent: #7467d9;
}

.preset-card--bass_impact {
  --preset-accent: #d95858;
}

.preset-symbol {
  position: relative;
  z-index: 1;
  display: grid;
  width: 54px;
  height: 54px;
  margin-bottom: $spacing-lg;
  border-radius: $radius-xl;
  background: color-mix(in srgb, var(--preset-accent) 18%, transparent);
  color: var(--preset-accent);
  font-size: $fs-2xl;
  place-items: center;
}

.preset-copy {
  position: relative;
  z-index: 1;

  h2 {
    margin: 0 0 $spacing-sm;
  }

  p {
    min-height: 3em;
    margin: 0 0 $spacing-lg;
    color: var(--text-secondary);
  }
}

.preset-counts {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: $spacing-sm;
}

@include mobile {
  .preset-grid {
    grid-template-columns: 1fr;
  }

  .preset-copy p {
    min-height: 0;
  }
}
</style>
