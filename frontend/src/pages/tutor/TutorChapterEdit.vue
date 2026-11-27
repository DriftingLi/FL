<template>
  <div class="pb-15">
    <UiAsyncSection
      :error="loadError"
      :loading="loading"
      :empty="isEmpty"
      :retrying="retrying"
      error-title="章节加载失败"
      error-description="未能读取章节内容，请检查网络后重试。"
      @retry="handleRetry"
    >
      <template #skeleton>
        <UiSkeleton variant="text" :rows="12" />
      </template>
      <template #empty>
        <UiEmptyState
          title="章节不存在或已删除"
          description="该章节可能已被移除，或链接中的章节编号有误。"
          action-text="返回章节列表"
          @action="goBackToChapters"
        />
      </template>

    <template v-if="chapterDetail">
      <UiPageHeader
        :title="chapterDetail.title"
        :subtitle="courseName || undefined"
        back
        @back="goBackToChapters"
      >
        <template #actions>
          <UiTag v-if="reviewDescriptor" :tone="reviewDescriptor.tone">
            {{ reviewDescriptor.label }}
          </UiTag>
          <UiButton
            size="small"
            :loading="submittingReview"
            :disabled="!canWrite || chapterDetail.review_status === 'pending'"
            @click="submitReview"
          >
            <UiIcon name="upload" /> 提交审核
          </UiButton>
          <UiButton size="small" :disabled="!canWrite" @click="openMetaDialog">
            <UiIcon name="pencil" /> 编辑信息
          </UiButton>
        </template>
      </UiPageHeader>

      <div
        v-if="chapterDetail.review_status === 'draft' && chapterDetail.reject_reason"
        class="mb-4 rounded-lg border border-bad-soft bg-bad-soft px-4 py-3 text-sm text-bad-strong"
      >
        上次驳回理由：{{ chapterDetail.reject_reason }}
      </div>

      <!-- 未认领降级（#2596 D3）：直达本页也可能没认领这门课。读面不受限（内容照常展示），
           但所有写操作禁用并说明原因；后端 403 兜底仍在（本条只是把失败提前说清楚）。 -->
      <div
        v-if="claimState === 'unclaimed'"
        class="mb-4 flex flex-wrap items-center justify-between gap-3 rounded-lg border border-warn/40 bg-warn-soft px-4 py-3 text-sm"
      >
        <span class="text-ink">认领本课程后才能编辑章节；现在可以先浏览内容</span>
        <UiButton size="small" variant="primary" :loading="claiming" @click="handleClaim">
          认领课程
        </UiButton>
      </div>

      <UiCard class="mb-6">
        <!-- ⚠️ 下划线必须用 \ 转义成 \_ ：Tailwind 任意变体里裸 _ 会被解析成空格，
             .el-tabs__header 会被拆成 `.el-tabs header` 后代选择器而选不中目标（构建期无报错）。 -->
        <el-tabs
          v-model="activeTab"
          class="[&_.el-tabs\_\_header]:sticky [&_.el-tabs\_\_header]:top-0 [&_.el-tabs\_\_header]:z-[2] [&_.el-tabs\_\_header]:mb-4 [&_.el-tabs\_\_header]:bg-panel"
        >
          <!-- 图文 Tab（始终显示，可编辑） -->
          <el-tab-pane label="图文" name="content">
            <div class="flex flex-col gap-3">
              <div class="flex items-center justify-between">
                <span class="text-sm text-ink-3">工作副本 · 使用 Markdown 编辑，支持实时预览</span>
                <div class="flex items-center gap-2">
                  <UiButton size="small" @click="openPreview">发布端预览</UiButton>
                  <UiButton
                    size="small"
                    :loading="savingContent"
                    :disabled="!canWrite || !contentChanged"
                    @click="saveContent"
                  >
                    <UiIcon name="check" /> 保存正文
                  </UiButton>
                </div>
              </div>
              <MarkdownEditor
                ref="markdownEditorRef"
                :key="chapterDetail.chapter_id"
                v-model="editContent"
                :height="560"
                :upload-url="chapterImageUploadUrl"
                content-styles
                placeholder="请输入章节正文内容（支持 Markdown 语法，可粘贴或上传图片）..."
              />
            </div>
          </el-tab-pane>

          <!-- 媒体类型 Tabs（按 TYPE_ORDER 顺序） -->
          <el-tab-pane v-for="group in fileGroups" :key="group.type" :name="group.type">
            <template #label>
              <span class="inline-flex items-center gap-1">
                <UiIcon :name="group.icon" :size="16" :class="group.iconClass" />
                {{ group.label }}
                <UiTag class="ml-1">{{ group.files.length }}</UiTag>
              </span>
            </template>

            <div class="flex flex-col gap-5">
              <!-- 已上传文件列表 -->
              <section class="rounded-ctl border border-line bg-canvas p-4">
                <div class="mb-3 flex items-center justify-between">
                  <h3 class="text-sm font-semibold text-ink">已上传文件（{{ group.files.length }}）</h3>
                  <div class="flex items-center gap-2">
                    <UiButton size="small" :disabled="!canWrite" @click="openLibraryPicker(group.type)">
                      <UiIcon name="folder-open" /> 从文件库挂接
                    </UiButton>
                    <UiButton size="small" :disabled="!canWrite" @click="openUploadDialog(group.type)">
                      <UiIcon name="upload" /> 上传{{ group.label }}
                    </UiButton>
                  </div>
                </div>

                <div v-if="group.files.length > 0" class="flex flex-col gap-2">
                  <div
                    v-for="file in group.files"
                    :key="file.file_id"
                    class="flex cursor-pointer items-center gap-2.5 rounded-ctl border border-line bg-panel px-3 py-2.5 transition-colors hover:border-ui-500"
                    :class="{ 'border-ui-500 bg-[color-mix(in_srgb,var(--color-ui-500)_12%,var(--color-panel))]': selectedFileKey === fileKey(file) }"
                    @click="selectFile(file)"
                  >
                    <UiIcon :name="group.icon" class="shrink-0 text-ui-soft-ink" :size="18" :class="group.iconClass" />
                    <div class="flex min-w-0 flex-1 flex-col gap-0.5">
                      <span class="truncate text-sm text-ink" :title="file.file_name">{{ file.file_name }}</span>
                      <span v-if="file.file_size" class="text-xs text-ink-3">{{ formatSize(file.file_size) }}</span>
                    </div>
                    <!-- 挂接文件库条目（source=tutor_file）：解挂/换文件（B6 换挂是章节侧动作）；
                         chapter_file 条目才是删除。两类 file_id 独立编号，动作面按 source 分流。 -->
                    <template v-if="file.source === 'tutor_file'">
                      <UiButton
                        variant="text"
                        size="small"
                        aria-label="换文件"
                        :disabled="!canWrite"
                        @click.stop="openLibraryPicker(group.type, file)"
                      >
                        换文件
                      </UiButton>
                      <UiButton
                        variant="text"
                        size="small"
                        class="text-bad"
                        :disabled="!canWrite"
                        aria-label="取消挂接"
                        @click.stop="handleDetachFile(file)"
                      >
                        解挂
                      </UiButton>
                    </template>
                    <UiButton
                      v-else
                      variant="text"
                      size="small"
                      class="text-bad"
                      :disabled="!canWrite"
                      aria-label="删除文件"
                      @click.stop="handleDeleteFile(file)"
                    >
                      <UiIcon name="trash-2" />
                    </UiButton>
                  </div>
                </div>

                <UiEmptyState v-else size="sm" :description="`暂无${group.label}文件`" />
              </section>

              <!-- 选中文件预览 -->
              <section v-if="selectedFile" class="rounded-ctl border border-line bg-panel p-4">
                <h3 class="mb-3 text-sm font-semibold text-ink">预览：{{ selectedFile.file_name }}</h3>
                <div class="min-h-[200px]">
                  <VideoPlayer v-if="group.type === 'video'" :src="selectedFile.file_url || ''" />
                  <DocumentViewer
                    v-else-if="group.type === 'document'"
                    :src="selectedFile.file_url || ''"
                    :file-name="selectedFile.file_name || ''"
                  />
                  <PptViewer
                    v-else-if="group.type === 'ppt'"
                    :src="selectedFile.file_url || ''"
                    :file-name="selectedFile.file_name || ''"
                    :chapter-id="chapterDetail.chapter_id"
                  />
                </div>
              </section>
            </div>
          </el-tab-pane>
        </el-tabs>
      </UiCard>

      <section v-if="reviewSnapshot" class="review-snapshot mt-4 rounded-card border border-line bg-panel p-4">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h3 class="text-sm font-semibold text-ink">在审快照</h3>
            <p class="mt-1 text-xs text-ink-3">本次送审时冻结的正文与文件；当前编辑只改动工作副本。</p>
          </div>
          <UiButton size="small" @click="openReviewSnapshotPreview">预览在审快照</UiButton>
        </div>
        <div class="mt-4 text-sm font-medium text-ink">冻结文件清单（{{ reviewSnapshot.files.length }}）</div>
        <ul v-if="reviewSnapshot.files.length" class="mt-2 list-disc pl-5 text-sm text-ink-3">
          <li v-for="file in reviewSnapshot.files" :key="`${file.source}-${file.file_id}`">{{ file.file_name }}</li>
        </ul>
        <div v-else class="mt-2 text-sm text-ink-3">本章节无附件</div>
      </section>
      <UiAlert
        v-else-if="chapterDetail.review_status === 'pending'"
        title="在审快照不可用"
        description="未能读取本次送审的冻结正文与文件；当前编辑器仅为工作副本，请刷新页面重试。"
        type="warning"
        :closable="false"
      />

      <!-- 上一章 / 下一章 -->
      <nav class="flex items-center justify-between gap-4 border-t border-line py-4 max-md:flex-col">
        <div class="nav-prev flex-1 max-w-[48%] max-md:max-w-full">
          <UiButton
            variant="text"
            :disabled="!chapterDetail.previous_chapter_id"
            @click="navigateToChapter(chapterDetail.previous_chapter_id)"
          >
            <UiIcon name="arrow-left" />
            <span v-if="chapterDetail.previous_chapter_id" class="flex flex-col items-start max-md:items-start">
              <span class="text-xs text-ink-3">上一章节</span>
              <span class="mt-0.5 text-sm font-medium text-ink">{{ getPrevChapterTitle }}</span>
            </span>
            <span v-else class="text-xs text-ink-3">没有上一章节</span>
          </UiButton>
        </div>

        <div class="nav-next flex-1 max-w-[48%] max-md:max-w-full">
          <UiButton
            variant="text"
            :disabled="!chapterDetail.next_chapter_id"
            @click="navigateToChapter(chapterDetail.next_chapter_id)"
          >
            <span v-if="chapterDetail.next_chapter_id" class="flex flex-col items-end max-md:items-start">
              <span class="text-xs text-ink-3">下一章节</span>
              <span class="mt-0.5 text-sm font-medium text-ink">{{ getNextChapterTitle }}</span>
            </span>
            <span v-else class="text-xs text-ink-3">没有下一章节</span>
            <UiIcon name="arrow-right" />
          </UiButton>
        </div>
      </nav>
    </template>
    </UiAsyncSection>

    <!-- 元信息编辑弹窗 -->
    <UiDialog v-model="metaDialogVisible" title="编辑章节信息" width="500px" confirm-text="保存" :confirm-loading="savingMeta" @confirm="saveMeta">
      <el-form :model="metaForm" label-width="100px">
        <el-form-item label="章节标题">
          <UiInput v-model="metaForm.title" placeholder="请输入章节标题" :maxlength="100" show-word-limit />
        </el-form-item>
        <el-form-item label="章节描述">
          <UiInput
            v-model="metaForm.description"
            type="textarea"
            :rows="3"
            placeholder="章节描述（可选）"
          />
        </el-form-item>
        <el-form-item label="预计时长">
          <el-input-number v-model="metaForm.duration" :min="0" :max="9999" />
          <span class="ml-2 text-sm text-ink-3">分钟</span>
        </el-form-item>
      </el-form>
    </UiDialog>

    <!--
      发布端预览（#903）：渲染走**发布端同一个实现**（PublishPreviewDialog → PublishMarkdown），
      不是 Vditor 内部 lute 引擎的解释——预览与发布同源才是这个入口的意义。
    -->
    <PublishPreviewDialog
      v-model="previewVisible"
      :content="previewContent"
      :title="previewSource === 'review' ? '在审快照预览' : '发布端预览'"
      :subtitle="previewSource === 'review' ? '送审时冻结的正文，与当前工作副本相互独立。' : '与 Web 学员端章节正文同一渲染器：表格 / 代码高亮 / 公式。'"
    />

    <!-- 上传文件弹窗 -->
    <UiDialog
      v-model="uploadDialogVisible"
      :title="`上传${uploadTypeLabel}文件`"
      width="720px"
      :close-on-click-modal="false"
      @close="handleUploadDialogClose"
    >
      <FileUpload
        v-if="uploadDialogVisible"
        ref="fileUploadRef"
        :chapter-id="chapterDetail?.chapter_id ?? 0"
        :initial-filter="uploadType"
        @upload-all="handleUploadAll"
      />
    </UiDialog>

    <!-- 从文件库挂接 / 换文件（#2497）：只列同类型、未挂接的库文件 -->
    <UiDialog
      v-model="libraryDialogVisible"
      :title="libraryMode === 'swap' ? '从文件库换文件' : '从文件库挂接'"
      width="640px"
    >
      <div v-if="libraryLoading" class="py-6 text-center text-sm text-ink-3">加载文件库…</div>
      <UiEmptyState v-else-if="libraryFiles.length === 0" size="sm" description="文件库中没有同类型的未挂接文件" />
      <div v-else class="flex flex-col gap-2">
        <div
          v-for="f in libraryFiles"
          :key="f.file_id"
          class="flex items-center gap-2.5 rounded-ctl border border-line bg-panel px-3 py-2.5"
        >
          <UiIcon name="file-text" class="shrink-0 text-ui-soft-ink" :size="18" />
          <div class="flex min-w-0 flex-1 flex-col">
            <span class="truncate text-sm text-ink" :title="f.file_name">{{ f.file_name }}</span>
            <span class="text-xs text-ink-3">{{ formatSize(f.file_size) }}</span>
          </div>
          <UiButton size="small" variant="primary" :disabled="!canWrite" @click="handleAttachLibrary(f)">
            {{ libraryMode === 'swap' ? '换入' : '挂接' }}
          </UiButton>
        </div>
      </div>
    </UiDialog>
  </div>
</template>

<script setup lang="ts">
import type { IconName } from '@/icons/registry'
import UiIcon from '@/components/ui/UiIcon.vue'
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { href } from '@/config/pages'
import { tutorApi, type TutorChapter } from '@/api/tutor'
import { tutorFileApi, type FileDTO } from '@/api/tutorfile'
import type { ChapterFile } from '@/api/course'
import MarkdownEditor from '@/components/tutor/MarkdownEditor.vue'
import PublishPreviewDialog from '@/components/render/PublishPreviewDialog.vue'
import FileUpload from '@/components/tutor/FileUpload.vue'
import VideoPlayer from '@/components/student/VideoPlayer.vue'
import DocumentViewer from '@/components/student/DocumentViewer.vue'
import PptViewer from '@/components/student/PptViewer.vue'
import UiPageHeader from '@/components/ui/UiPageHeader.vue'
import UiCard from '@/components/ui/UiCard.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiTag from '@/components/ui/UiTag.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'
import UiAsyncSection from '@/components/ui/UiAsyncSection.vue'
import UiEmptyState from '@/components/ui/UiEmptyState.vue'
import UiAlert from '@/components/ui/UiAlert.vue'
import { useAsyncPage } from '@/composables/useAsyncPage'
import { useChapterEditingSession } from './useChapterEditingSession'
import UiDialog from '@/components/ui/UiDialog.vue'
import UiInput from '@/components/ui/UiInput.vue'
import { useConfirm } from '@/composables/useConfirm'

const route = useRoute()
const router = useRouter()
const session = useChapterEditingSession()
const {
  chapterDetail,
  editContent,
  chapterFiles,
  reviewSnapshot,
  activeTab,
  selectedFileKey,
  selectedFile,
  claimState,
  claiming,
  canWrite,
  contentChanged,
  savingContent,
  submittingReview,
  saveContent,
  fileKey,
  selectFile
} = session

function handleClaim() {
  void session.claimCourse(Number(route.params.courseId))
}

const courseName = ref('')
const chapters = ref<TutorChapter[]>([])

// 发布端预览（#903）：取编辑器里的**最新**正文快照渲染，不写回表单、不发请求。
// 不直接用 v-model：ir 模式下 v-model 偶发滞后一拍，预览会显示上一版内容——
// 而「预览不准」正是这个入口要消灭的问题。
const markdownEditorRef = ref<{ getValue: () => string } | null>(null)
const previewVisible = ref(false)
const previewContent = ref('')
const previewSource = ref<'working' | 'review'>('working')

function openPreview() {
  previewSource.value = 'working'
  previewContent.value = markdownEditorRef.value?.getValue() || editContent.value
  previewVisible.value = true
}

function openReviewSnapshotPreview() {
  const snapshot = reviewSnapshot.value
  if (!snapshot) return
  previewSource.value = 'review'
  previewContent.value = snapshot.content
  previewVisible.value = true
}

// 文件分组（图片不再作为独立章节文件上传，统一走图文 Markdown 粘贴）
const TYPE_ORDER = ['video', 'document', 'ppt']

/**
 * 媒体类型 → 图标与图标色调。
 * 色调改用语义 token：视频沿用原有红色系（bad）、PPT 沿用橙黄（warn），
 * 文档由 Element 默认蓝改为品牌石墨青 ui-500。
 */
const TYPE_CONFIG: Record<string, { label: string; icon: IconName; iconClass: string }> = {
  video: { label: '视频', icon: 'video', iconClass: 'text-bad' },
  document: { label: '文档', icon: 'file-text', iconClass: 'text-ui-soft-ink' },
  ppt: { label: 'PPT', icon: 'file-text', iconClass: 'text-warn' }
}

// Vditor 图片上传走 /api/tutor/upload-image（返回 Vditor 期望的 {code,msg,data:{succMap}} 格式）。
// 携带 chapter_id 按章节分目录存储 images/chapters/<chapterId>/，删除章节时可按前缀清理。
const chapterImageUploadUrl = computed(() => {
  const chapterId = chapterDetail.value?.chapter_id
  return chapterId ? `/api/tutor/upload-image?chapter_id=${chapterId}` : '/api/tutor/upload-image'
})

const fileGroups = computed(() => {
  const groups: Record<string, ChapterFile[]> = {}
  for (const file of chapterFiles.value) {
    const type = file.content_type || 'document'
    if (!groups[type]) groups[type] = []
    groups[type].push(file)
  }
  // 导师端始终展示所有媒体类型 tab，即使没有文件也可切换进入上传
  return TYPE_ORDER.map((type) => ({
    type,
    label: TYPE_CONFIG[type]?.label || type,
    icon: TYPE_CONFIG[type]?.icon || 'file-text',
    iconClass: TYPE_CONFIG[type]?.iconClass || 'text-ink-3',
    files: groups[type] || []
  }))
})

// 元信息编辑
const metaDialogVisible = ref(false)
const savingMeta = ref(false)
const metaForm = ref({ title: '', description: '', duration: 0 })

function openMetaDialog() {
  if (!chapterDetail.value) return
  metaForm.value = {
    title: chapterDetail.value.title || '',
    description: chapterDetail.value.description || '',
    duration: chapterDetail.value.duration || 0
  }
  metaDialogVisible.value = true
}

async function saveMeta() {
  if (!chapterDetail.value) return
  if (!metaForm.value.title.trim()) {
    ElMessage.warning('章节标题不能为空')
    return
  }
  savingMeta.value = true
  try {
    await tutorApi.updateChapter(chapterDetail.value.chapter_id, {
      title: metaForm.value.title.trim(),
      description: metaForm.value.description,
      duration: metaForm.value.duration
    })
    chapterDetail.value.title = metaForm.value.title.trim()
    chapterDetail.value.description = metaForm.value.description
    chapterDetail.value.duration = metaForm.value.duration
    ElMessage.success('章节信息更新成功')
    metaDialogVisible.value = false
  } catch {
    /* 错误已由拦截器提示 */
  } finally {
    savingMeta.value = false
  }
}

// #2498 C1/C2：审核状态与显式「提交审核」动作（保存只落草稿，提交冻结整章快照入队）。
const reviewDescriptor = computed<{ label: string; tone: 'warning' | 'info' | 'success' } | null>(() => {
  switch (chapterDetail.value?.review_status) {
    case 'draft':
      return { label: '草稿', tone: 'warning' }
    case 'pending':
      return { label: '审核中', tone: 'info' }
    case 'published':
      return { label: '已发布', tone: 'success' }
    default:
      return null
  }
})

function submitReview() {
  void session.submitReview(() => useConfirm().confirm(
    '提交后将冻结已保存正文与文件清单进入审核（后续改动不影响在审版本），确认提交？',
    '提交审核'
  ))
}

// 上传弹窗
const uploadDialogVisible = ref(false)
const uploadType = ref('')
const fileUploadRef = ref<InstanceType<typeof FileUpload> | null>(null)

const uploadTypeLabel = computed(() => {
  if (!uploadType.value) return ''
  return TYPE_CONFIG[uploadType.value]?.label || ''
})

function openUploadDialog(type: string) {
  uploadType.value = type
  uploadDialogVisible.value = true
}

function handleUploadDialogClose() {
  fileUploadRef.value?.resetState()
}

interface UploadAllResult {
  total: number
  success: number
  failed: number
}

function handleUploadAll(result: UploadAllResult) {
  if (result.failed === 0) {
    ElMessage.success(`全部上传成功，共${result.total}个文件`)
  } else {
    ElMessage.warning(`上传完成：成功${result.success}个，失败${result.failed}个`)
  }
  // 上传完成后立即刷新章节详情，让用户看到新文件
  void session.refreshFiles()
}

// 删除文件
async function handleDeleteFile(file: ChapterFile) {
  try {
    await useConfirm().confirmDanger(
      `确定要删除文件"${file.file_name}"吗？`,
      '确认删除',
      { confirmButtonText: '确定', cancelButtonText: '取消', type: 'warning' }
    )
    if (file.file_id != null) {
      await tutorApi.deleteFile(file.file_id)
    }
    ElMessage.success('文件删除成功')
    if (selectedFileKey.value === fileKey(file)) {
      selectedFileKey.value = null
    }
    await session.refreshFiles()
  } catch {
    /* 取消确认或删除失败；错误已由拦截器提示 */
  }
}

// 从文件库挂接 / 换文件（#2497 spec B2/B6）：picker 只列同类型、未挂接的库文件；
// 挂接=attachFile，换文件=swapChapterFile（挂新 + 旧回库一步原子，B6：换挂是章节侧动作）。
const libraryDialogVisible = ref(false)
const libraryMode = ref<'attach' | 'swap'>('attach')
const libraryFileType = ref('video')
const librarySwapOldId = ref<number | null>(null)
const libraryFiles = ref<FileDTO[]>([])
const libraryLoading = ref(false)

async function openLibraryPicker(type: string, swapOld?: ChapterFile) {
  libraryFileType.value = type
  libraryMode.value = swapOld ? 'swap' : 'attach'
  librarySwapOldId.value = swapOld?.file_id ?? null
  libraryFiles.value = []
  libraryDialogVisible.value = true
  libraryLoading.value = true
  try {
    libraryFiles.value = (await tutorFileApi.listFiles({ content_type: type, attached: false })).files
  } catch {
    /* 错误已由拦截器提示 */
  } finally {
    libraryLoading.value = false
  }
}

async function handleAttachLibrary(file: FileDTO) {
  const chapterId = chapterDetail.value?.chapter_id
  if (!chapterId) return
  try {
    if (libraryMode.value === 'swap' && librarySwapOldId.value != null) {
      await tutorFileApi.swapChapterFile(chapterId, librarySwapOldId.value, file.file_id)
      ElMessage.success('已换文件，旧文件回到文件库')
    } else {
      await tutorFileApi.attachFile(file.file_id, chapterId)
      ElMessage.success('已挂接到本章节')
    }
    libraryDialogVisible.value = false
    await session.refreshFiles()
  } catch {
    /* 错误已由拦截器提示 */
  }
}

async function handleDetachFile(file: ChapterFile) {
  try {
    await useConfirm().confirm(`取消挂接「${file.file_name}」？文件将回到文件库未挂状态。`, '取消挂接')
    await tutorFileApi.detachFile(file.file_id)
    ElMessage.success('已取消挂接，文件回到文件库')
    if (selectedFileKey.value === fileKey(file)) {
      selectedFileKey.value = null
    }
    await session.refreshFiles()
  } catch {
    /* 取消确认或解挂失败；错误已由拦截器提示 */
  }
}

// 数据加载：三态收编 useAsyncPage（#401，详情页无分页 / #1101）——loader 只管拉数据与写响应；
// 「404 = 空态、其余 = 错误态」由 useAsyncPage 的 isEmpty / loadError 判定
// （复用 ApiErrorKind），页面不自建 chapterNotFound；错误详情由拦截器统一 toast。
const { loading, loadError, retrying, isEmpty, retry: handleRetry, run: loadChapterDetail } = useAsyncPage(
  async () => {
    const chapterId = Number(route.params.chapterId)
    const res = await tutorApi.getChapterDetail(chapterId)
    // 默认 tab：图文优先，否则第一个媒体 tab
    return res
  },
  {
    apply: (res) => {
      session.loadChapter(res)
    }, itemsRef: chapterDetail }
)

async function loadCourseInfo() {
  try {
    const courseId = Number(route.params.courseId)
    const res = await tutorApi.getCourseChapters(courseId)
    courseName.value = res.course?.name || ''
    chapters.value = res.chapters || []
  } catch (e) {
    // 静默失败：课程名/上下章标题缺失不影响正文编辑
    console.error('Failed to load course info:', e)
  }
}

watch(() => route.params.courseId, () => { void loadCourseInfo() }, { immediate: true })

const getPrevChapterTitle = computed(() => {
  const detail = chapterDetail.value
  if (!detail?.previous_chapter_id) return ''
  const prev = chapters.value.find((c) => c.chapter_id === detail.previous_chapter_id)
  return prev ? prev.title : ''
})

const getNextChapterTitle = computed(() => {
  const detail = chapterDetail.value
  if (!detail?.next_chapter_id) return ''
  const next = chapters.value.find((c) => c.chapter_id === detail.next_chapter_id)
  return next ? next.title : ''
})

function navigateToChapter(chapterId?: number | null) {
  if (!chapterId) return
  const courseId = route.params.courseId
  router.push({
    name: 'TutorChapterEdit',
    params: { courseId, chapterId }
  })
}

function goBackToChapters() {
  const courseId = route.params.courseId
  router.push(href('TutorChapterManage', { id: String(courseId) }))
}

function formatSize(bytes: number) {
  if (!bytes) return ''
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return (bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0) + ' ' + units[i]
}

// 切换章节时重新加载
watch(
  () => route.params.chapterId,
  (newId) => {
    if (newId) {
      loadChapterDetail()
    }
  }
)

// 切换 Tab 时重置选中文件，避免跨 Tab 预览错误
watch(activeTab, () => {
  selectedFileKey.value = null
})

onMounted(() => {
  loadChapterDetail()
  void session.loadClaimState(Number(route.params.courseId))
})
</script>

<style scoped>
/*
 * 上一章/下一章导航（#2540）。
 *
 * 仅保留 :deep 的 EP 内部覆盖（R1 允许，无法用原子类表达）：
 * - 导航按钮需要更大的可点击区（padding 12/16，高度自适应）
 * - 「下一章」贴容器右边缘：`.el-button` 是 inline-flex、容器默认 block，
 *   此时 `margin-left: auto` **不生效**（auto 只对 flex 子项生效）——必须把容器
 *   变成 flex 再 justify-content: flex-end。
 *
 * 窄屏（max-md）由模板上的 max-md:flex-col 变成纵向堆叠，两条导航不再分左右，
 * 下面的 media query 把贴右收回，避免窄屏偏右。
 */
.nav-prev :deep(.el-button),
.nav-next :deep(.el-button) {
  padding: 12px 16px;
  height: auto;
}

.nav-prev :deep(.el-button) {
  text-align: left;
}

.nav-next {
  display: flex;
  justify-content: flex-end;
}

.nav-next :deep(.el-button) {
  text-align: right;
}

@media (max-width: 767px) {
  .nav-next {
    justify-content: flex-start;
  }

  .nav-next :deep(.el-button) {
    text-align: left;
  }

  .nav-next :deep(.el-button .flex-col) {
    align-items: flex-start;
  }
}
</style>
