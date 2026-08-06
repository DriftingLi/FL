<template>
  <div class="forum-image-uploader">
    <el-upload
      v-model:file-list="fileList"
      list-type="picture-card"
      :limit="Math.max(props.max - props.modelValue.length, 0)"
      :accept="accept"
      :http-request="handleUpload"
      :on-remove="handleRemove"
      :before-upload="beforeUpload"
      :disabled="uploading"
    >
      <el-icon v-if="uploading"><Loading /></el-icon>
      <el-icon v-else><Plus /></el-icon>
    </el-upload>

    <p class="upload-tip">最多 {{ props.max }} 张图片，单张不超过 20MB</p>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import type { UploadFile, UploadRequestOptions } from 'element-plus'
import { ElMessage } from 'element-plus'
import { Plus, Loading } from '@element-plus/icons-vue'
import { forumApi } from '@/api/forum'

const props = withDefaults(defineProps<{
  /** 已上传成功的图片 URL 数组（v-model） */
  modelValue: string[]
  /** 图片数量上限 */
  max?: number
}>(), {
  max: 9
})

const emit = defineEmits(['update:modelValue'])

const accept = 'image/*'
const uploading = ref(false)

const fileList = ref<UploadFile[]>([])

// 外部重置（发帖/回复成功后清空）
watch(() => props.modelValue, (val) => {
  if (val.length === 0 && fileList.value.length > 0) {
    fileList.value = []
  }
})

// 上传前校验：单张大小上限 20MB（与后端 ValidateImageFile 对齐）
function beforeUpload(file: File) {
  if (file.size > 20 * 1024 * 1024) {
    ElMessage.error('单张图片不能超过 20MB')
    return false
  }
  return true
}

// 自定义上传：调论坛上传接口拿 URL，push 到 modelValue
async function handleUpload(options: UploadRequestOptions) {
  const formData = new FormData()
  formData.append('file', options.file)
  uploading.value = true
  try {
    const res = await forumApi.uploadImage(formData)
    if ((res.code === 200 || res.code === 201) && res.data?.url) {
      const url = res.data.url
      // 让 picture-card 展示缩略图
      const file = fileList.value.find(f => f.uid === options.file.uid)
      if (file) {
        ;(file as any).url = url
      }
      emit('update:modelValue', [...props.modelValue, url])
      options.onSuccess?.(res)
    } else {
      ElMessage.error(res.message || '图片上传失败')
      options.onError?.(new Error(res.message || '图片上传失败') as any)
    }
  } catch (e: any) {
    ElMessage.error(e.message || '图片上传失败')
    options.onError?.(e)
  } finally {
    uploading.value = false
  }
}

// 删除图片：从 URL 数组剔除（后端由悬空图片定时任务回收存储文件）
function handleRemove(file: UploadFile) {
  const url = (file as any).url as string | undefined
  if (url && props.modelValue.includes(url)) {
    emit('update:modelValue', props.modelValue.filter(u => u !== url))
  }
}
</script>

<style scoped>
.forum-image-uploader {
  width: 100%;
}

.forum-image-uploader :deep(.el-upload--picture-card) {
  width: 100px;
  height: 100px;
}

.upload-tip {
  margin: 8px 0 0;
  font-size: 12px;
  color: #909399;
}
</style>
