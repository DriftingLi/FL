<template>
  <el-form :label-width="labelWidth" @submit.prevent>
    <el-form-item label="标题" required>
      <el-input v-model="form.title" maxlength="100" show-word-limit :placeholder="titlePlaceholder" />
    </el-form-item>
    <el-form-item :label="contentLabel" required>
      <el-input
        v-model="form.content"
        type="textarea"
        :rows="contentRows"
        maxlength="10000"
        show-word-limit
        :placeholder="contentPlaceholder"
      />
    </el-form-item>
    <el-form-item label="图片">
      <ForumImageUploader v-model="form.images" :max="9" />
    </el-form-item>
  </el-form>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { forumApi, type ForumCategory } from '@/api/forum'
import ForumImageUploader from './ForumImageUploader.vue'

/**
 * 发帖表单体（#389）：论坛页对话框与问答整页两种壳共享字段 / 校验 / 长度限制 / 提交，
 * category 参数化 —— 壳只保留自己的形态（弹层 / 整页）与发布后的跳转。
 * 提交与状态经 defineExpose 暴露（壳的按钮在表单体之外）。
 */
const props = withDefaults(defineProps<{
  /** 帖子类别（#364）：判"帖子意图"的唯一依据，随 createTopic 提交 */
  category: ForumCategory
  /** 内容字段标签：对话框「内容」/ 问答页「正文」，校验文案同步使用 */
  contentLabel?: string
  /** 正文行数 */
  contentRows?: number
  /** 标签宽（两壳栅格不同） */
  labelWidth?: string
  titlePlaceholder?: string
  contentPlaceholder?: string
  successMessage?: string
}>(), {
  contentLabel: '内容',
  contentRows: 8,
  labelWidth: '70px',
  titlePlaceholder: '请输入标题（1-100 字）',
  contentPlaceholder: '请输入内容（1-10000 字）',
  successMessage: '发布成功'
})

const emit = defineEmits<{ success: [] }>()

const form = ref<{ title: string; content: string; images: string[] }>({ title: '', content: '', images: [] })
const submitting = ref(false)
const canSubmit = computed(() => form.value.title.trim().length > 0 && form.value.content.trim().length > 0)

function reset() {
  form.value = { title: '', content: '', images: [] }
}

/** 校验 + 提交。成功返回 true（壳据此关壳/跳转），失败不抛错（拦截器已 toast）。 */
async function submit(): Promise<boolean> {
  const title = form.value.title.trim()
  const content = form.value.content.trim()
  if (!title || !content) {
    ElMessage.warning(`请填写标题和${props.contentLabel}`)
    return false
  }
  if (title.length > 100) {
    ElMessage.warning('标题不能超过 100 字')
    return false
  }
  if (content.length > 10000) {
    ElMessage.warning(`${props.contentLabel}不能超过 10000 字`)
    return false
  }
  submitting.value = true
  try {
    await forumApi.createTopic({ category: props.category, title, content, images: form.value.images })
    ElMessage.success(props.successMessage)
    reset()
    emit('success')
    return true
  } catch {
    /* 错误已由拦截器提示 */
    return false
  } finally {
    submitting.value = false
  }
}

defineExpose({ canSubmit, submitting, submit, reset })
</script>
