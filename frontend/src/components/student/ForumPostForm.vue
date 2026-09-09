<script setup lang="ts">
/**
 * 发帖表单体（#389）：论坛页对话框与问答整页两种壳共享字段 / 校验 / 长度限制 / 提交，
 * category 参数化 —— 壳只保留自己的形态（弹层 / 整页）与发布后的跳转。
 * 提交与状态经 defineExpose 暴露（壳的按钮在表单体之外）。
 *
 * 布局：从 el-form 的左置 label 改为 label 上置的轻量表单。
 * 左置 label 是管理后台的语汇，放在学员端发帖场景里观感偏「填报表单」。
 *
 * 类别 chips（#742 批次四）：传入 categories 时渲染三分类切换（默认 = category prop，
 * 可切换），并联动内容区 placeholder 与一行功能性提示——源头降低发错分区的概率；
 * 不传（或章节帖 chapter_id 通道）不渲染，存量调用零 diff。
 *
 * ⚠️ 对外契约不可变 —— ForumAskPage.vue 与 ForumPage.vue 依赖
 * defineExpose({ canSubmit, submitting, submit, reset })，改任一项都会让壳的按钮失效。
 */
import { computed, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { forumApi, type ForumCategory } from '@/api/forum'
import UiInput from '@/components/ui/UiInput.vue'
import UiSegmentTabs from '@/components/ui/UiSegmentTabs.vue'
import ForumImageUploader from './ForumImageUploader.vue'

const props = withDefaults(defineProps<{
  /** 帖子类别（#364）：判"帖子意图"的唯一依据，随 createTopic 提交；chips 激活时作为默认选中值 */
  category: ForumCategory
  /**
   * 类别 chips 可选项（#742 批次四）。传入即渲染类别切换（通常传三分类全量）；
   * 不传不渲染，存量调用零 diff。章节帖（chapterId 通道）强制不渲染——两者互斥。
   */
  categories?: ForumCategory[]
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

// ===== 类别 chips（#742 批次四）=====

const CATEGORY_OPTIONS: Array<{ label: string; value: ForumCategory }> = [
  { label: '讨论', value: 'discussion' },
  { label: '问答', value: 'question' },
  { label: '备考经验', value: 'experience' }
]

/** chips 是否渲染：壳传了 categories 且非章节帖通道（chapter_id 与 category 互斥） */
const chipsActive = computed(() => Array.isArray(props.categories) && props.categories.length > 0 && props.chapterId == null)

/** 当前选中的类别：默认 = category prop（所在 Tab），可切换 */
const selectedCategory = ref<ForumCategory>(props.category)

// 入口类别快照（#742）：placeholder/提示仅在「用户主动切换且偏离入口类别」时联动——
// 切回入口类别即恢复壳传入的定制文案（保护问答页定制 placeholder 不被永久覆盖）
const entryCategory = ref<ForumCategory>(props.category)
const userTouchedCategory = ref(false)

// 壳的默认类别随 Tab 变化（弹窗开着切 Tab 的场景）时视为新一轮入口并同步选中值
watch(() => props.category, (next) => {
  entryCategory.value = next
  selectedCategory.value = next
})

// 类别联动提示（#742）：按类别给 placeholder 与一行功能性提示，讨论维持通用文案
const CATEGORY_HINTS: Record<ForumCategory, { contentPlaceholder: string; hint: string }> = {
  discussion: {
    contentPlaceholder: '请输入内容（1-10000 字）',
    hint: ''
  },
  question: {
    contentPlaceholder: '请描述问题现象、已尝试的做法与期望结果（越具体越容易被采纳）',
    hint: '问题描述越具体，越容易被采纳'
  },
  experience: {
    contentPlaceholder: '建议写清：考试批次/科目、备考经过、可复用的建议',
    hint: '写清批次/科目、经过与可复用的建议，帮后来人少走弯路'
  }
}

/** 是否已偏离入口类别（用户主动切换且当前选中 ≠ 入口类别） */
const categoryDeviated = computed(
  () => chipsActive.value && userTouchedCategory.value && selectedCategory.value !== entryCategory.value
)

/** placeholder 联动：偏离入口类别后用类别文案；未偏离严格沿用壳传入值（存量调用零 diff） */
const effectiveContentPlaceholder = computed(() =>
  categoryDeviated.value ? CATEGORY_HINTS[selectedCategory.value].contentPlaceholder : props.contentPlaceholder
)
/** 一行功能性提示：仅在偏离入口类别且该类别有提示时显示 */
const activeHint = computed(() =>
  categoryDeviated.value ? CATEGORY_HINTS[selectedCategory.value].hint : ''
)

const chipOptions = computed(() =>
  (props.categories ?? []).map((c) => CATEGORY_OPTIONS.find((o) => o.value === c) ?? { label: c, value: c })
)

function handleCategoryChange(v: string) {
  userTouchedCategory.value = true
  selectedCategory.value = v as ForumCategory
}

function reset() {
  form.value = { title: '', content: '', images: [] }
  entryCategory.value = props.category
  selectedCategory.value = props.category
  userTouchedCategory.value = false
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
    // 普通帖走 category 通道（#742 起类别可经 chips 切换，默认仍 = 壳传入的 category）
    // —— 两者互斥，与改造前两种提交形态一一对应
    const payload =
      props.chapterId != null
        ? { chapter_id: props.chapterId, title, content, images: form.value.images }
        : { category: selectedCategory.value, title, content, images: form.value.images }
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
    <div v-if="chipsActive">
      <label class="mb-1.5 block text-sm font-medium text-ink">
        <span class="mr-0.5 text-bad">*</span>类别
      </label>
      <UiSegmentTabs
        :model-value="selectedCategory"
        :options="chipOptions"
        @update:model-value="handleCategoryChange"
      />
    </div>

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
        :placeholder="effectiveContentPlaceholder"
      />
      <p v-if="activeHint" class="mt-1.5 mb-0 text-xs text-ink-3">{{ activeHint }}</p>
    </div>

    <div>
      <label class="mb-1.5 block text-sm font-medium text-ink">图片</label>
      <ForumImageUploader v-model="form.images" :max="9" />
    </div>
  </div>
</template>
