<template>
  <div v-if="images && images.length > 0" class="forum-image-gallery">
    <el-image
      v-for="(url, index) in images"
      :key="url + index"
      :src="resolveFileUrl(url)"
      :preview-src-list="previewList"
      :initial-index="index"
      fit="cover"
      class="gallery-image"
      preview-teleported
    />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { resolveFileUrl } from '@/utils/fileUrl'

const props = defineProps<{
  images?: string[]
}>()

const previewList = computed(() => (props.images || []).map(u => resolveFileUrl(u)))
</script>

<style scoped>
.forum-image-gallery {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 10px;
}

.gallery-image {
  width: 100px;
  height: 100px;
  border-radius: 6px;
  cursor: zoom-in;
  flex-shrink: 0;
}
</style>
