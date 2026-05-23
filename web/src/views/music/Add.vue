<script setup lang="ts">
import { ref } from "vue"
import BatchMusicUpload from "@/components/upload/BatchMusicUpload.vue"
import SingleMusicUpload from "@/components/upload/SingleMusicUpload.vue"

const activeTab = ref("single")
</script>

<template>
  <div class="add-page">
    <el-card class="content-card" shadow="never">
      <el-tabs v-model="activeTab">
        <el-tab-pane :label="$t('add.single_import')" name="single">
          <SingleMusicUpload />
        </el-tab-pane>
        <el-tab-pane :label="$t('add.batch_import')" name="batch">
          <BatchMusicUpload />
        </el-tab-pane>
      </el-tabs>
    </el-card>
  </div>
</template>

<style scoped lang="scss">
.add-page {
  padding: 20px 0;
}
</style>
