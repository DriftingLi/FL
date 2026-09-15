<script setup lang="ts">
/**
 * 论坛输入区（发帖回复共用）：回复胶囊 + 正文输入框 + 正文格式 + 属地披露 + 提交。
 *
 * 收编前，ForumDetail 与 ChapterDiscussion 各自写了一遍几乎相同的回复框
 * （内容 / 图片 / 回复目标 / 上传 / 提交），却长成两套样式。这里合成一个，
 * 上传状态机与输入框形态各自单点：`useForumImageUpload`（#389）管上传，
 * `ForumMarkdownInput`（#1017）管形态。
 *
 * #1017 起本组件**不再自己摆图片入口与缩略图**：图二那块虚线粘贴区在
 * `ForumMarkdownInput` 里，发帖表单与回复框共用同一个。同理，粘贴监听不再是
 * document 级（原来要靠 stopPropagation 防止一次粘贴被两处各传一遍）——
 * 现在是「卡片内粘贴 → 转发给上传单点」，两处同时挂载也不会互相抢事件。
 *
 * ⚠️ Tailwind 坑：本项目不引 preflight，浏览器默认 border-width 是 medium(3px)。
 * 因此凡是写 border-* 样式类的地方，必须显式给出宽度工具类（border / border-t），
 * 否则没宽度的边会被渲染成 3px。
 */
import { computed, ref } from 'vue'
import { Promotion } from '@element-plus/icons-vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiTag from '@/components/ui/UiTag.vue'
import UiSegmentTabs from '@/components/ui/UiSegmentTabs.vue'
import ForumMarkdownInput from './ForumMarkdownInput.vue'
import { useForumContentFormat, FORUM_FORMAT_OPTIONS } from '@/composables/useForumContentFormat'
import { FORUM_REGION_NOTICE } from '@/utils/forumDisplay'
import type { ForumContentFormat } from '@/api/forum'

const props = withDefaults(
  defineProps<{
    /** 输入内容 */
    modelValue: string
    /** 已上传成功的图片 URL 列表 */
    images: string[]
    /** 正在回复的对象；为 null 时不显示胶囊 */
    replyingTo?: { id: number; username: string } | null
    /** 提交中（由父级持有，避免组件自己管请求） */
    submitting?: boolean
    /** 图片张数上限 */
    maxImages?: number
    /** 内容最大长度 */
    maxlength?: number
    /** textarea 行数 */
    rows?: number
    placeholder?: string
  }>(),
  {
    replyingTo: null,
    submitting: false,
    maxImages: 3,
    maxlength: 5000,
    rows: 3,
    placeholder: '写下你的回复…'
  }
)

const emit = defineEmits<{
  'update:modelValue': [string]
  'update:images': [string[]]
  'update:replyingTo': [{ id: number; username: string } | null]
  /**
   * 提交。带上正文格式：格式状态与发帖侧共用同一偏好，
   * 父级调 replyTopic 时需要它——不带上父级就只能猜。
   */
  submit: [{ contentFormat: ForumContentFormat }]
}>()

const content = computed({
  get: () => props.modelValue,
  set: (v: string) => emit('update:modelValue', v)
})

// 受控回写父级的 images：上传成功与删除都经由 ForumMarkdownInput 透传上来
const images = computed({
  get: () => props.images,
  set: (v: string[]) => emit('update:images', v)
})

// ===== 正文格式（#879 / ADR-0044）=====
// 与发帖侧**共用同一个偏好键与同一套切换态**（composable 单点）：
// 同一个人对正文格式的偏好与他写的是主题还是回复无关。
const { format: contentFormat, handleFormatChange } = useForumContentFormat()

/** 提交口径沿用改造前：内容非空或图片非空 */
const canSubmit = computed(() => content.value.trim().length > 0 || props.images.length > 0)

const inputRef = ref<{ resetPreview?: () => void } | null>(null)

function submit() {
  if (!canSubmit.value || props.submitting) return
  emit('submit', { contentFormat: contentFormat.value })
  // 提交后父级会清空正文：此时若仍停在预览态，用户看到的是一块空预览、
  // 还得手动点「编写」才能写下一句。复位到编写态（与发帖表单 reset 同口径）。
  inputRef.value?.resetPreview?.()
}

/** Ctrl / Cmd + Enter 发送 */
function onKeydown(event: KeyboardEvent) {
  if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') {
    event.preventDefault()
    submit()
  }
}
</script>

<template>
  <div class="forum-composer">
    <div v-if="props.replyingTo" class="mb-2">
      <UiTag closable size="small" tone="info" @close="emit('update:replyingTo', null)">
        回复 @{{ props.replyingTo.username }}
      </UiTag>
    </div>

    <ForumMarkdownInput
      ref="inputRef"
      v-model="content"
      v-model:images="images"
      :format="contentFormat"
      :max-images="props.maxImages"
      :maxlength="props.maxlength"
      :rows="props.rows"
      :placeholder="props.placeholder"
      @keydown="onKeydown"
    />

    <!-- 正文格式：与发帖侧同控件同口径（数据档位，与输入框内的视图档位分工见 UiUnderlineTabs） -->
    <div class="mt-2 flex flex-wrap items-center gap-2">
      <UiSegmentTabs
        :model-value="contentFormat"
        :options="FORUM_FORMAT_OPTIONS"
        @update:model-value="handleFormatChange"
      />
    </div>

    <!-- 发布前披露（ADR-0045）：ui-conventions「不写说明性 hint」的明确例外，
         文案单点在 forumDisplay.FORUM_REGION_NOTICE —— 别在这里另抄一份。 -->
    <p class="forum-region-notice mt-1.5 mb-0 text-xs text-ink-3">{{ FORUM_REGION_NOTICE }}</p>

    <div class="mt-2 flex items-center justify-end gap-2">
      <UiButton
        variant="primary"
        :icon="Promotion"
        circle
        size="small"
        :loading="props.submitting"
        :disabled="!canSubmit"
        title="发表回复（Ctrl / Cmd + Enter）"
        @click="submit"
      />
    </div>
  </div>
</template>
