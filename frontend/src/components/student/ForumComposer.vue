<script setup lang="ts">
/**
 * 论坛输入区（发帖回复共用）
 *
 * 收编前，ForumDetail 与 ChapterDiscussion 各自写了一遍几乎相同的回复框
 * （内容 / 图片 / 回复目标 / 上传 / 提交），却长成两套样式。这里合成一个，
 * 上传状态机仍复用 useForumImageUpload（#389 单点），本组件只负责形态。
 *
 * 形态要点：外层卡片承担边框与聚焦反馈（focus-within），内层文本框走
 * UiInput 的 bare 变体去掉自身边框 —— 高亮落在外框上，而不是文本框内部。
 *
 * ⚠️ Tailwind 坑：本项目不引 preflight，浏览器默认 border-width 是 medium(3px)。
 * 因此凡是写 border-* 样式类的地方，必须显式给出宽度工具类（border / border-t），
 * 否则没宽度的边会被渲染成 3px。
 */
import { computed, ref } from 'vue'
import { Picture, Promotion, Close } from '@element-plus/icons-vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiInput from '@/components/ui/UiInput.vue'
import { useForumImageUpload } from '@/composables/useForumImageUpload'
import { resolveFileUrl } from '@/utils/fileUrl'

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
  submit: []
}>()

const content = computed({
  get: () => props.modelValue,
  set: (v: string) => emit('update:modelValue', v)
})

// 受控回写父级的 images：上传成功与删除都经由 useForumImageUpload 写回这里
const images = computed({
  get: () => props.images,
  set: (v: string[]) => emit('update:images', v)
})

const { uploading, uploadFiles, removeImage, handlePaste } = useForumImageUpload(
  () => props.maxImages,
  { urls: images }
)

const fileInput = ref<HTMLInputElement | null>(null)

/** 提交口径沿用改造前：内容非空或图片非空 */
const canSubmit = computed(() => content.value.trim().length > 0 || props.images.length > 0)

function submit() {
  if (!canSubmit.value || props.submitting) return
  emit('submit')
}

function triggerSelect() {
  fileInput.value?.click()
}

function onFileChange(event: Event) {
  const target = event.target as HTMLInputElement
  const files = Array.from(target.files ?? [])
  if (files.length > 0) void uploadFiles(files)
  // 清空以便重复选择同一文件
  target.value = ''
}

/**
 * 粘贴上传。
 *
 * 额外 stopPropagation 的原因：ForumImageUploader 是在 **document** 上监听 paste 的，
 * 而章节讨论里回复框（本组件）与发帖表单（含 ForumImageUploader）会同时挂载 ——
 * 不拦住冒泡，同一次粘贴会被两处各上传一遍，图片凭空多出一倍。
 */
function onPaste(event: ClipboardEvent) {
  handlePaste(event)
  event.stopPropagation()
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
      <el-tag closable size="small" type="info" @close="emit('update:replyingTo', null)">
        回复 @{{ props.replyingTo.username }}
      </el-tag>
    </div>

    <!--
      粘贴监听挂在卡片上而非 document：只有焦点在本输入框内粘贴图片才接管，
      避免用户在页面其他输入框粘贴时被抢走。
    -->
    <div
      class="rounded-[10px] border border-line bg-panel p-3 transition-colors duration-[var(--duration-base)] ease-[var(--ease-default)] focus-within:border-ui-500"
      @paste="onPaste"
    >
      <div v-if="props.images.length > 0" class="mb-2 flex flex-wrap gap-1.5">
        <div
          v-for="(url, index) in props.images"
          :key="url + index"
          class="relative size-12 shrink-0 overflow-hidden rounded-[6px] border border-line"
        >
          <el-image :src="resolveFileUrl(url)" fit="cover" class="h-full w-full" />
          <button
            type="button"
            class="absolute right-0 top-0 flex size-4 items-center justify-center rounded-bl-[6px] border-0 bg-black/55 p-0 text-[10px] text-panel hover:bg-bad/90"
            @click="removeImage(index)"
          >
            <el-icon><Close /></el-icon>
          </button>
        </div>
      </div>

      <UiInput
        v-model="content"
        type="textarea"
        variant="bare"
        :rows="props.rows"
        :maxlength="props.maxlength"
        :placeholder="props.placeholder"
        @keydown="onKeydown"
      />

      <div class="mt-2 flex items-center gap-2 border-t border-line pt-2">
        <UiButton
          variant="text"
          :icon="Picture"
          circle
          size="small"
          :loading="uploading"
          :disabled="props.images.length >= props.maxImages"
          title="添加图片（也可直接粘贴）"
          @click="triggerSelect"
        />
        <span class="text-xs text-ink-3">{{ props.images.length }}/{{ props.maxImages }}</span>

        <span class="ml-auto text-xs text-ink-3">{{ content.length }}/{{ props.maxlength }}</span>
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

    <input
      ref="fileInput"
      type="file"
      accept="image/*"
      multiple
      class="hidden"
      @change="onFileChange"
    />
  </div>
</template>
