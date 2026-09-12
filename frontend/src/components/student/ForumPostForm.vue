<script setup lang="ts">
/**
 * 发帖表单体（#389）：论坛页对话框与问答整页两种壳共享字段 / 校验 / 长度限制 / 提交，
 * category 参数化 —— 壳只保留自己的形态（弹层 / 整页）与发布后的跳转。
 * 提交与状态经 defineExpose 暴露（壳的按钮在表单体之外）。
 *
 * 布局：从 el-form 的左置 label 改为 label 上置的轻量表单。
 * 左置 label 是管理后台的语汇，放在学员端发帖场景里观感偏「填报表单」。
 *
 * 类别 chips（#742 批次四，ADR-0040 收窄）：传入 categories 时渲染**自述意图**切换（讨论/问答，
 * 默认 = category prop 归一后的值），并联动内容区 placeholder 与一行功能性提示。
 * 「备考经验」改为管理端认定后已从发帖/编辑入参收窄——传进来的历史 experience 一律归一为
 * discussion，既不渲染成选项也选不回去，学员端发布入口不会撞上后端 400。
 * 不传（或章节帖 chapter_id 通道）不渲染，存量调用零 diff。
 *
 * ⚠️ 对外契约不可变 —— ForumAskPage.vue 与 ForumPage.vue 依赖
 * defineExpose({ canSubmit, submitting, submit, reset })，改任一项都会让壳的按钮失效。
 */
import { computed, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { forumApi, type ForumCategory, type ForumPublishCategory } from '@/api/forum'
import UiInput from '@/components/ui/UiInput.vue'
import UiSegmentTabs from '@/components/ui/UiSegmentTabs.vue'
import UiButton from '@/components/ui/UiButton.vue'
import ForumImageUploader from './ForumImageUploader.vue'
import ForumContent from './ForumContent.vue'
import { useForumContentFormat, FORUM_FORMAT_OPTIONS } from '@/composables/useForumContentFormat'

const props = withDefaults(defineProps<{
  /**
   * 帖子类别（#364）：判"帖子意图"的唯一依据；chips 激活时作为默认选中值。
   * ADR-0040 起发布侧只认 discussion/question：传入历史 'experience'（如经验 Tab 下开的壳）
   * 会被归一为 'discussion'，保证 createTopic 永不产出该值。
   */
  category: ForumCategory
  /**
   * 类别 chips 可选项（#742 批次四）。传入即渲染类别切换；认得的值只有讨论/问答，
   * 历史 'experience' 被忽略（不渲染成选项，也选不回去）；不传不渲染，存量调用零 diff。
   * 章节帖（chapterId 通道）强制不渲染——两者互斥。
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

// ===== 正文格式（#878 / ADR-0044）=====
// 首次默认纯文本、之后记住上次选择；声明位置于「作者自述」，不由系统猜测。
// 偏好与切换态（选项/预览/切换处理）收在 composable 一处，与回复框共用同一套口径。
const { format: contentFormat, isMarkdown, previewing, handleFormatChange, resetPreview } = useForumContentFormat()
const canSubmit = computed(() => form.value.title.trim().length > 0 && form.value.content.trim().length > 0)

// ===== 类别 chips（#742 批次四）=====

const CATEGORY_OPTIONS: Array<{ label: string; value: ForumPublishCategory }> = [
  { label: '讨论', value: 'discussion' },
  { label: '问答', value: 'question' }
]

/** chips 选项：只放学员能自述的意图（ADR-0040），历史 'experience' 不入选项 */
const chipOptions = computed(() =>
  (props.categories ?? [])
    .map((c) => CATEGORY_OPTIONS.find((o) => o.value === c))
    .filter((o): o is { label: string; value: ForumPublishCategory } => o != null)
)

/** chips 是否渲染：有可选项且非章节帖通道（chapter_id 与 category 互斥） */
const chipsActive = computed(() => chipOptions.value.length > 0 && props.chapterId == null)

/** 发布侧意图归一（ADR-0040）：'experience'（历史值 / 经验 Tab 入口）落 'discussion' */
function toPublishCategory(c: ForumCategory): ForumPublishCategory {
  return c === 'question' ? 'question' : 'discussion'
}

/** 当前选中的类别：默认 = category prop（所在 Tab）归一后的意图，可切换 */
const selectedCategory = ref<ForumPublishCategory>(toPublishCategory(props.category))

// 入口类别快照（#742）：placeholder/提示仅在「用户主动切换且偏离入口类别」时联动——
// 切回入口类别即恢复壳传入的定制文案（保护问答页定制 placeholder 不被永久覆盖）
const entryCategory = ref<ForumPublishCategory>(toPublishCategory(props.category))
const userTouchedCategory = ref(false)

// 壳的默认类别随 Tab 变化（弹窗开着切 Tab 的场景）时视为新一轮入口并同步选中值
watch(() => props.category, (next) => {
  const normalized = toPublishCategory(next)
  entryCategory.value = normalized
  selectedCategory.value = normalized
})

// 类别联动提示（#742）：按类别给 placeholder 与一行功能性提示，讨论维持通用文案
const CATEGORY_HINTS: Record<ForumPublishCategory, { contentPlaceholder: string; hint: string }> = {
  discussion: {
    contentPlaceholder: '请输入内容（1-10000 字）',
    hint: ''
  },
  question: {
    contentPlaceholder: '请描述问题现象、已尝试的做法与期望结果（越具体越容易被采纳）',
    hint: '问题描述越具体，越容易被采纳'
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

function handleCategoryChange(v: string) {
  userTouchedCategory.value = true
  selectedCategory.value = toPublishCategory(v as ForumCategory)
}

function reset() {
  form.value = { title: '', content: '', images: [] }
  // 格式是**用户偏好**不是本次输入，reset 不重置它（重开表单仍是他上次的选择）
  resetPreview()
  entryCategory.value = toPublishCategory(props.category)
  selectedCategory.value = entryCategory.value
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
    // 普通帖走 category 通道（#742 起类别可经 chips 切换，默认 = 壳传入类别归一后的意图，
    // ADR-0040 起只可能是 discussion/question）—— 两者互斥，与改造前两种提交形态一一对应
    const payload =
      props.chapterId != null
        ? { chapter_id: props.chapterId, title, content, images: form.value.images, content_format: contentFormat.value }
        : { category: selectedCategory.value, title, content, images: form.value.images, content_format: contentFormat.value }
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
      <label class="mb-1.5 block text-sm font-medium text-ink">正文格式</label>
      <UiSegmentTabs :model-value="contentFormat" :options="FORUM_FORMAT_OPTIONS" @update:model-value="handleFormatChange" />
    </div>

    <div>
      <label class="mb-1.5 flex items-center gap-2 text-sm font-medium text-ink">
        <span><span class="mr-0.5 text-bad">*</span>{{ contentLabel }}</span>
        <!-- 预览与发布同源：都走 ForumContent 这一个渲染单点，不另起一套预览渲染 -->
        <UiButton v-if="isMarkdown" variant="text" size="small" class="ml-auto" @click="previewing = !previewing">
          {{ previewing ? '继续编辑' : '预览' }}
        </UiButton>
      </label>
      <ForumContent
        v-if="previewing"
        :content="form.content"
        format="markdown"
        class="min-h-[120px] rounded-[6px] border border-line bg-canvas p-3 text-sm leading-[1.7] text-ink"
      />
      <UiInput
        v-else
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
