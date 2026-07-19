<script setup lang="ts">
import type { MusicMetadataFields } from "@/types/api";

type MetadataField = keyof MusicMetadataFields;

const props = defineProps<{
  modelValue: MusicMetadataFields;
}>();

const emit = defineEmits<{
  touch: [field: MetadataField];
  "update:modelValue": [value: MusicMetadataFields];
}>();

const update = <K extends MetadataField>(field: K, value: MusicMetadataFields[K]) => {
  emit("update:modelValue", { ...props.modelValue, [field]: value });
  emit("touch", field);
};

const updateGenres = (genres: string[]) => {
  emit("update:modelValue", { ...props.modelValue, genres: [...genres], genre: genres.join("; ") });
  emit("touch", "genres");
  emit("touch", "genre");
};
</script>

<template>
  <div class="metadata-editor">
    <el-row :gutter="20">
      <el-col :xs="24" :sm="12">
        <el-form-item :label="$t('add.music_title')" prop="title">
          <el-input :model-value="modelValue.title" @update:model-value="update('title', $event)" />
        </el-form-item>
      </el-col>
      <el-col :xs="24" :sm="12">
        <el-form-item :label="$t('add.music_artist')" prop="artist">
          <el-input :model-value="modelValue.artist" @update:model-value="update('artist', $event)" />
        </el-form-item>
      </el-col>
    </el-row>

    <el-row :gutter="20">
      <el-col :xs="24" :sm="12">
        <el-form-item :label="$t('add.music_album')">
          <el-input :model-value="modelValue.album" @update:model-value="update('album', $event)" />
        </el-form-item>
      </el-col>
      <el-col :xs="24" :sm="12">
        <el-form-item :label="$t('add.album_artist')">
          <el-input :model-value="modelValue.album_artist" @update:model-value="update('album_artist', $event)" />
        </el-form-item>
      </el-col>
    </el-row>

    <el-row :gutter="20">
      <el-col :xs="12" :sm="6">
        <el-form-item :label="$t('add.music_year')">
          <el-input :model-value="modelValue.year" placeholder="2024" @update:model-value="update('year', $event)" />
        </el-form-item>
      </el-col>
      <el-col :xs="12" :sm="6">
        <el-form-item :label="$t('add.music_track')">
          <el-input :model-value="modelValue.track" placeholder="1" @update:model-value="update('track', $event)" />
        </el-form-item>
      </el-col>
      <el-col :xs="12" :sm="6">
        <el-form-item :label="$t('add.track_total')">
          <el-input :model-value="modelValue.track_total" placeholder="12" @update:model-value="update('track_total', $event)" />
        </el-form-item>
      </el-col>
      <el-col :xs="12" :sm="6">
        <el-form-item :label="$t('add.music_duration')">
          <el-input :model-value="modelValue.duration" placeholder="03:45" @update:model-value="update('duration', $event)" />
        </el-form-item>
      </el-col>
    </el-row>

    <el-form-item :label="$t('add.music_genres')">
      <el-select
        :model-value="modelValue.genres"
        multiple
        filterable
        allow-create
        default-first-option
        :reserve-keyword="false"
        :placeholder="$t('add.list_field_hint')"
        @update:model-value="updateGenres"
      />
    </el-form-item>

    <details class="advanced-metadata">
      <summary>{{ $t("add.advanced_metadata") }}</summary>

      <el-row :gutter="20">
        <el-col :xs="24" :sm="12">
          <el-form-item :label="$t('add.artists')">
            <el-select
              :model-value="modelValue.artists"
              multiple
              filterable
              allow-create
              default-first-option
              :reserve-keyword="false"
              :placeholder="$t('add.list_field_hint')"
              @update:model-value="update('artists', $event)"
            />
          </el-form-item>
        </el-col>
        <el-col :xs="24" :sm="12">
          <el-form-item :label="$t('add.album_artists')">
            <el-select
              :model-value="modelValue.album_artists"
              multiple
              filterable
              allow-create
              default-first-option
              :reserve-keyword="false"
              :placeholder="$t('add.list_field_hint')"
              @update:model-value="update('album_artists', $event)"
            />
          </el-form-item>
        </el-col>
      </el-row>

      <el-row :gutter="20">
        <el-col :xs="12" :sm="6">
          <el-form-item :label="$t('add.disc_number')">
            <el-input :model-value="modelValue.disc" placeholder="1" @update:model-value="update('disc', $event)" />
          </el-form-item>
        </el-col>
        <el-col :xs="12" :sm="6">
          <el-form-item :label="$t('add.disc_total')">
            <el-input :model-value="modelValue.disc_total" placeholder="2" @update:model-value="update('disc_total', $event)" />
          </el-form-item>
        </el-col>
        <el-col :xs="24" :sm="6">
          <el-form-item :label="$t('add.release_date')">
            <el-input
              :model-value="modelValue.release_date"
              :placeholder="$t('add.partial_date_hint')"
              @update:model-value="update('release_date', $event)"
            />
          </el-form-item>
        </el-col>
        <el-col :xs="24" :sm="6">
          <el-form-item :label="$t('add.original_release_date')">
            <el-input
              :model-value="modelValue.original_release_date"
              :placeholder="$t('add.partial_date_hint')"
              @update:model-value="update('original_release_date', $event)"
            />
          </el-form-item>
        </el-col>
      </el-row>

      <el-form-item :label="$t('add.tag_comment')">
        <el-input
          :model-value="modelValue.comment"
          type="textarea"
          :rows="2"
          @update:model-value="update('comment', $event)"
        />
      </el-form-item>

      <el-form-item label="ISRC">
        <el-select
          :model-value="modelValue.isrcs"
          multiple
          filterable
          allow-create
          default-first-option
          :reserve-keyword="false"
          :placeholder="$t('add.list_field_hint')"
          @update:model-value="update('isrcs', $event)"
        />
      </el-form-item>

      <el-divider content-position="left">MusicBrainz</el-divider>
      <el-row :gutter="20">
        <el-col :xs="24" :sm="12">
          <el-form-item :label="$t('add.mb_recording_id')">
            <el-input
              :model-value="modelValue.musicbrainz_recording_id"
              @update:model-value="update('musicbrainz_recording_id', $event)"
            />
          </el-form-item>
        </el-col>
        <el-col :xs="24" :sm="12">
          <el-form-item :label="$t('add.mb_track_id')">
            <el-input
              :model-value="modelValue.musicbrainz_track_id"
              @update:model-value="update('musicbrainz_track_id', $event)"
            />
          </el-form-item>
        </el-col>
      </el-row>
      <el-row :gutter="20">
        <el-col :xs="24" :sm="12">
          <el-form-item :label="$t('add.mb_release_id')">
            <el-input
              :model-value="modelValue.musicbrainz_release_id"
              @update:model-value="update('musicbrainz_release_id', $event)"
            />
          </el-form-item>
        </el-col>
        <el-col :xs="24" :sm="12">
          <el-form-item :label="$t('add.mb_release_group_id')">
            <el-input
              :model-value="modelValue.musicbrainz_release_group_id"
              @update:model-value="update('musicbrainz_release_group_id', $event)"
            />
          </el-form-item>
        </el-col>
      </el-row>
      <el-row :gutter="20">
        <el-col :xs="24" :sm="12">
          <el-form-item :label="$t('add.mb_artist_ids')">
            <el-select
              :model-value="modelValue.musicbrainz_artist_ids"
              multiple
              filterable
              allow-create
              default-first-option
              :reserve-keyword="false"
              :placeholder="$t('add.list_field_hint')"
              @update:model-value="update('musicbrainz_artist_ids', $event)"
            />
          </el-form-item>
        </el-col>
        <el-col :xs="24" :sm="12">
          <el-form-item :label="$t('add.mb_album_artist_ids')">
            <el-select
              :model-value="modelValue.musicbrainz_album_artist_ids"
              multiple
              filterable
              allow-create
              default-first-option
              :reserve-keyword="false"
              :placeholder="$t('add.list_field_hint')"
              @update:model-value="update('musicbrainz_album_artist_ids', $event)"
            />
          </el-form-item>
        </el-col>
      </el-row>
    </details>
  </div>
</template>

<style scoped lang="scss">
.metadata-editor {
  :deep(.el-select) {
    width: 100%;
  }
}

.advanced-metadata {
  margin: 0 0 $spacing-lg;
  padding: $spacing-sm $spacing-md $spacing-md;
  border: 1px solid var(--border-color);
  border-radius: $radius-md;

  summary {
    padding: $spacing-sm 0;
    color: var(--text-dark);
    font-weight: $fw-semibold;
    cursor: pointer;
  }
}
</style>
