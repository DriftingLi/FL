<template>
  <div class="forum-image-uploader">
    <!-- 已选图片缩略图 -->
    <div v-if="props.modelValue.length > 0" class="img-thumbs">
      <div v-for="(url, index) in props.modelValue" :key="url + index" class="img-thumb">
        <el-image :src="resolveFileUrl(url)" fit="cover" class="thumb-img" />
        <button type="button" class="thumb-remove" @click="removeImage(index)">
          <el-icon><Close /></el-icon>
        </button>
      </div>
    </div>

    <!-- 上传入口：小图标按钮（达到上限后隐藏） -->
    <div class="upload-row">
      <button
        v-if="props.modelValue.length < props.max"
        type="button"
        class="upload-btn"
        :disabled="uploading"
        title="添加图片（也可直接粘贴图片）"
        @click="triggerSelect"
      >
        <el-icon class="btn-icon" :class="{ spinning: uploading }">
          <Loading v-if="uploading" />
          <Picture v-else />
        </el-icon>
        <span class="btn-count" v-if="props.modelValue.length > 0">
          {{ props.modelValue.length }}/{{ props.max }}
        </span>
      </button>
    </div>

    <input
      ref="fileInput"
      type="file"
      :accept="accept"
      multiple
      style="display: none"
      @change="handleSelect"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { Loading, Picture, Close } from '@element-plus/icons-vue'
import { resolveFileUrl } from '@/utils/fileUrl'
import { useForumImageUpload } from '@/composables/useForumImageUpload'

const props = withDefaults(defineProps<{
  /** 已上传成功的图片 URL 数组（v-model） */
  modelValue: string[]
  /** 图片数量上限 */
  max?: number
}>(), {
  max: 9
})

const emit = defineEmits(['update:modelValue'])

// 上传校验与状态机进 useForumImageUpload（#389 单点）：URL 列表经可写 computed 受控回写父级
const urls = computed({
  get: () => props.modelValue,
  set: v => emit('update:modelValue', v)
})
const { uploading, uploadFiles, removeImage, handlePaste } = useForumImageUpload(() => props.max, { urls })

const accept = 'image/*'
const fileInput = ref<HTMLInputElement | null>(null)

function triggerSelect() {
  fileInput.value?.click()
}

// 选择文件（含多选）
function handleSelect(event: Event) {
  const target = event.target as HTMLInputElement
  const files = Array.from(target.files ?? [])
  if (files.length > 0) {
    void uploadFiles(files)
  }
  target.value = ''
}

onMounted(() => {
  document.addEventListener('paste', handlePaste)
})

onBeforeUnmount(() => {
  document.removeEventListener('paste', handlePaste)
})
</script>

<style scoped>
.forum-image-uploader {
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

/* 已选图片缩略图 */
.img-thumbs {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.img-thumb {
  position: relative;
  width: 48px;
  height: 48px;
  border-radius: 6px;
  overflow: hidden;
  border: 1px solid #ebeef5;
  flex-shrink: 0;
}

.thumb-img {
  width: 100%;
  height: 100%;
}

.thumb-remove {
  position: absolute;
  top: 0;
  right: 0;
  width: 16px;
  height: 16px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: none;
  border-radius: 0 0 0 6px;
  background: rgba(0, 0, 0, 0.55);
  color: #fff;
  cursor: pointer;
  padding: 0;
  font-size: 10px;
}

.thumb-remove:hover {
  background: rgba(245, 108, 108, 0.9);
}

/* 上传入口 */
.upload-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.upload-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 5px 10px;
  border: 1px dashed #c0c4cc;
  border-radius: 6px;
  background: #fafbfc;
  color: #606266;
  cursor: pointer;
  transition: all 0.2s;
}

.upload-btn:hover {
  border-color: #409eff;
  color: #409eff;
  background: #ecf5ff;
}

.upload-btn:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.btn-icon {
  font-size: 16px;
}

.btn-count {
  font-size: 12px;
  color: #909399;
}

.spinning {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}
</style>
