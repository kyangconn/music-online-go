<script setup lang="ts">
import { VideoPlay, StarFilled } from "@element-plus/icons-vue"
import { ElMessage } from "element-plus"
import { ref, watch } from "vue"
import { useRoute } from "vue-router"
import type { Music, PaginatedData } from "@/types/api"
import request from "@/utils/request"

const route = useRoute()
const musicList = ref<Music[]>([])
const loading = ref(false)
const searchQuery = ref("")
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(12)

const fetchMusic = async () => {
  loading.value = true
  try {
    const params = {
      q: searchQuery.value,
      page: currentPage.value,
      page_size: pageSize.value,
    }
    const res = await request.get<PaginatedData<Music>>("/musics", { params })
    musicList.value = res.data.items || []
    total.value = res.data.total
  } catch (error) {
    console.error(error)
    ElMessage.error("Failed to load music list")
  } finally {
    loading.value = false
  }
}

watch(
  () => route.query.q,
  (newQ) => {
    searchQuery.value = (newQ as string) || ""
    currentPage.value = 1
    fetchMusic()
  },
  { immediate: true },
)

const handlePageChange = (page: number) => {
  currentPage.value = page
  fetchMusic()
}
</script>

<template>
  <div class="home-container">
    <div class="banner">
      <h1>{{ $t("nav.discover") }}</h1>
      <p>{{ $t("nav.discover_desc") }}</p>
    </div>

    <div class="music-section">
      <h2 v-if="searchQuery">{{ $t("common.search_results_for", { query: searchQuery }) }}</h2>
      <h2 v-else>{{ $t("nav.recommended") }}</h2>

      <div v-if="loading" class="loading-container">
        <el-skeleton :rows="3" animated />
      </div>

      <div v-else-if="musicList.length === 0" class="empty-state">
        <el-empty :description="$t('common.no_music_found')" />
      </div>

      <div v-else class="music-grid-tight">
        <el-card
          v-for="music in musicList"
          :key="music.id"
          class="music-card"
          :body-style="{ padding: '0px' }"
          shadow="hover"
        >
          <div class="cover-image">
            <el-image
              :src="music.cover_url || 'https://via.placeholder.com/300x300?text=Music'"
              fit="cover"
              loading="lazy"
            >
              <template #error>
                <div class="image-slot">
                  <el-icon><icon-picture /></el-icon>
                </div>
              </template>
            </el-image>
            <div class="play-overlay">
              <router-link :to="`/music/${music.id}`">
                <el-button type="primary" circle :icon="VideoPlay" size="large" />
              </router-link>
            </div>
          </div>
          <div class="music-info">
            <h3 class="music-title" :title="music.title">{{ music.title }}</h3>
            <p class="music-artist">{{ music.artist }}</p>
            <div class="music-meta">
              <el-icon class="like-icon"><StarFilled /></el-icon>
              <span class="like-count">{{ music.like_count ?? 0 }}</span>
            </div>
          </div>
        </el-card>
      </div>

      <div class="pagination-container" v-if="total > pageSize">
        <el-pagination
          background
          layout="prev, pager, next"
          :total="total"
          :page-size="pageSize"
          v-model:current-page="currentPage"
          @current-change="handlePageChange"
        />
      </div>
    </div>
  </div>
</template>

<style scoped lang="scss">
.banner {
  background: linear-gradient(135deg, var(--secondary-color), var(--primary-color));
  color: white;
  padding: 3rem 2rem;
  border-radius: $radius-md;
  margin-bottom: 2rem;
  text-align: center;

  h1 {
    margin: 0 0 1rem;
    font-size: $fs-hero;
  }
  p {
    margin: 0;
    opacity: 0.8;
    font-size: $fs-xl;
  }
}

.music-card {
  transition: transform $transition-slow;
  border: none;
  background: var(--bg-white);

  &:hover {
    transform: translateY(-5px);
  }
}

.cover-image {
  @include aspect-ratio;
  .el-image {
    @include aspect-ratio-inner;
  }
}

.play-overlay {
  @include overlay-hover;
}

.music-card:hover .play-overlay {
  opacity: 1;
}

.music-info {
  padding: $spacing-md;
}

.music-title {
  margin: 0 0 5px;
  font-size: $fs-md;
  @include text-ellipsis;
  color: var(--text-dark);
}

.music-artist {
  margin: 0;
  font-size: $fs-sm;
  color: var(--text-light);
}

.music-meta {
  margin-top: 6px;
  @include inline-flex($spacing-xs);
  font-size: $fs-sm;
  color: var(--text-light);
}

.like-icon {
  color: $color-gold;
}

.pagination-container {
  @include flex-center;
  margin-top: 2rem;
}

.image-slot {
  @include flex-center;
  width: 100%;
  height: 100%;
  background: $color-image-slot-bg;
  color: $color-image-slot-text;
}
</style>
