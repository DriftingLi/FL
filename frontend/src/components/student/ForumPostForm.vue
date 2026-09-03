<script setup lang="ts">
/**
 * 发帖表单体（#389）：论坛页对话框与问答整页两种壳共享字段 / 校验 / 长度限制 / 提交，
 * category 参数化 —— 壳只保留自己的形态（弹层 / 整页）与发布后的跳转。
 * 提交与状态经 defineExpose 暴露（壳的按钮在表单体之外）。
 *
 * 布局：从 el-form 的左置 label 改为 label 上置的轻量表单。
 * 左置 label 是管理后台的语汇，放在学员端发帖场景里观感偏「填报表单」。
 *
 * ⚠️ 对外契约不可变 —— ForumAskPage.vue 与 ForumPage.vue 依赖
 * defineExpose({ canSubmit, submitting, submit, reset })，改任一项都会让壳的按钮失效。
 */
import { computed, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { forumApi, type ForumCategory } from '@/api/forum'
import UiInput from '@/components/ui/UiInput.vue'
import ForumImageUploader from './ForumImageUploader.vue'

const props = withDefaults(defineProps<{
  /** 帖子类别（#364）：判"帖子意图"的唯一依据，随 createTopic 提交 */
  category: ForumCategory
  /**
   * 章节讨论帖的章节 ID。传入后 createTopic 走 chapter_id 通道（不带 category），
   * 与章节讨论页共用本表单；不传（默认）保持 category 通道，存量调用零 diff。
   */
  chapterId?: number
  /** 内容字段标签：对话框「内容」/ 问答页「正文」，校验文案同步使用 */
  contentLabel?: string
  /** 正文行数 */
  contentRows?: number
  /**
   * @deprecated 改为 label 上置后已无栅格可对齐，保留仅为兼容存量调用方
   * （ForumAskPage.vue 传 56px、ForumPage.vue 走默认 70px），不参与渲染。
   */
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
    // 章节帖走 chapter_id 通道（后端以 chapter_id 判定 scope=chapter），
    // 普通帖走 category 通道 —— 两者互斥，与改造前两种提交形态一一对应
    const payload =
      props.chapterId != null
        ? { chapter_id: props.chapterId, title, content, images: form.value.images }
        : { category: props.category, title, content, images: form.value.images }
    await forumApi.createTopic(payload)
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

<template>
  <div class="forum-post-form flex flex-col gap-4">
    <div>
      <label class="mb-1.5 block text-sm font-medium text-ink">
        <span class="mr-0.5 text-bad">*</span>标题
      </label>
      <UiInput
        v-model="form.title"
        :maxlength="100"
        show-word-limit
        :placeholder="titlePlaceholder"
      />
    </div>

    <div>
      <label class="mb-1.5 block text-sm font-medium text-ink">
        <span class="mr-0.5 text-bad">*</span>{{ contentLabel }}
      </label>
      <UiInput
        v-model="form.content"
        type="textarea"
        :rows="contentRows"
        :maxlength="10000"
        show-word-limit
        :placeholder="contentPlaceholder"
      />
    </div>

    <div>
      <label class="mb-1.5 block text-sm font-medium text-ink">图片</label>
      <ForumImageUploader v-model="form.images" :max="9" />
    </div>
  </div>
</template>
