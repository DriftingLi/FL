<template>
  <div class="pb-15">
    <UiEmptyState
      v-if="chapterNotFound"
      title="章节不存在或已删除"
      description="该章节可能已被移除，或链接中的章节编号有误。"
      action-text="返回章节列表"
      @action="goBackToChapters"
    />

    <UiErrorState
      v-else-if="loadError"
      title="章节加载失败"
      description="未能读取章节内容，请检查网络后重试。"
      :retrying="retrying"
      @retry="handleRetry"
    />

    <UiSkeleton v-else-if="loading" variant="text" :rows="12" />

    <template v-else-if="chapterDetail">
      <UiPageHeader
        :title="chapterDetail.title"
        :subtitle="courseName || undefined"
        back
        @back="goBackToChapters"
      >
        <template #actions>
          <UiButton size="small" @click="openMetaDialog">
            <el-icon><Edit /></el-icon> 编辑信息
          </UiButton>
        </template>
      </UiPageHeader>

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
                <span class="text-[13px] text-ink-3">使用 Markdown 编辑，支持实时预览</span>
                <UiButton
                  size="small"
                  :loading="savingContent"
                  :disabled="!contentChanged"
                  @click="saveContent"
                >
                  <el-icon><Check /></el-icon> 保存正文
                </UiButton>
              </div>
              <MarkdownEditor
                :key="chapterDetail.chapter_id"
                v-model="editContent"
                :height="560"
                :upload-url="chapterImageUploadUrl"
                placeholder="请输入章节正文内容（支持 Markdown 语法，可粘贴或上传图片）..."
              />
            </div>
          </el-tab-pane>

          <!-- 媒体类型 Tabs（按 TYPE_ORDER 顺序） -->
          <el-tab-pane v-for="group in fileGroups" :key="group.type" :name="group.type">
            <template #label>
              <span class="inline-flex items-center gap-1">
                <el-icon :size="16" :class="group.iconClass">
                  <component :is="group.icon" />
                </el-icon>
                {{ group.label }}
                <UiTag class="ml-1">{{ group.files.length }}</UiTag>
              </span>
            </template>

            <div class="flex flex-col gap-5">
              <!-- 已上传文件列表 -->
              <section class="rounded-ctl border border-line bg-canvas p-4">
                <div class="mb-3 flex items-center justify-between">
                  <h3 class="text-sm font-semibold text-ink">已上传文件（{{ group.files.length }}）</h3>
                  <UiButton size="small" @click="openUploadDialog(group.type)">
                    <el-icon><Upload /></el-icon> 上传{{ group.label }}
                  </UiButton>
                </div>

                <div v-if="group.files.length > 0" class="flex flex-col gap-2">
                  <div
                    v-for="file in group.files"
                    :key="file.file_id"
                    class="flex cursor-pointer items-center gap-2.5 rounded-ctl border border-line bg-panel px-3 py-2.5 transition-colors hover:border-ui-500"
                    :class="{ 'border-ui-500 bg-ui-50': selectedFileId === file.file_id }"
                    @click="selectFile(file.file_id)"
                  >
                    <el-icon class="shrink-0 text-ui-500" :size="18" :class="group.iconClass">
                      <component :is="group.icon" />
                    </el-icon>
                    <div class="flex min-w-0 flex-1 flex-col gap-0.5">
                      <span class="truncate text-sm text-ink" :title="file.file_name">{{ file.file_name }}</span>
                      <span v-if="file.file_size" class="text-xs text-ink-3">{{ formatSize(file.file_size) }}</span>
                    </div>
                    <UiButton
                      variant="text"
                      size="small"
                      class="text-bad"
                      @click.stop="handleDeleteFile(file)"
                    >
                      <el-icon><Delete /></el-icon>
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

      <!-- 上一章 / 下一章 -->
      <nav class="flex items-center justify-between gap-4 border-t border-line py-4 max-md:flex-col">
        <div class="flex-1 max-w-[48%] max-md:max-w-full max-md:text-left">
          <UiButton
            variant="text"
            :disabled="!chapterDetail.previous_chapter_id"
            @click="navigateToChapter(chapterDetail.previous_chapter_id)"
          >
            <el-icon><ArrowLeft /></el-icon>
            <span v-if="chapterDetail.previous_chapter_id" class="flex flex-col items-start max-md:items-start">
              <span class="text-xs text-ink-3">上一章节</span>
              <span class="mt-0.5 text-sm font-medium text-ink">{{ getPrevChapterTitle }}</span>
            </span>
            <span v-else class="text-xs text-ink-3">没有上一章节</span>
          </UiButton>
        </div>

        <div class="flex-1 max-w-[48%] max-md:max-w-full max-md:text-left">
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
            <el-icon><ArrowRight /></el-icon>
          </UiButton>
        </div>
      </nav>
    </template>

    <!-- 元信息编辑弹窗 -->
    <el-dialog v-model="metaDialogVisible" title="编辑章节信息" width="500px">
      <el-form :model="metaForm" label-width="100px">
        <el-form-item label="章节标题">
          <el-input v-model="metaForm.title" placeholder="请输入章节标题" maxlength="100" show-word-limit />
        </el-form-item>
        <el-form-item label="章节描述">
          <el-input
            v-model="metaForm.description"
            type="textarea"
            :rows="3"
            placeholder="章节描述（可选）"
          />
        </el-form-item>
        <el-form-item label="预计时长">
          <el-input-number v-model="metaForm.duration" :min="0" :max="9999" />
          <span class="ml-2 text-[13px] text-ink-3">分钟</span>
        </el-form-item>
      </el-form>
      <template #footer>
        <UiButton @click="metaDialogVisible = false">取消</UiButton>
        <UiButton variant="primary" :loading="savingMeta" @click="saveMeta">保存</UiButton>
      </template>
    </el-dialog>

    <!-- 上传文件弹窗 -->
    <el-dialog
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
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch, type Component } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  ArrowLeft, ArrowRight, Edit, Check, Upload, Delete,
  VideoCamera, Document
} from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { tutorApi, type TutorChapter, type TutorChapterDetail } from '@/api/tutor'
import type { ChapterFile } from '@/api/course'
import MarkdownEditor from '@/components/tutor/MarkdownEditor.vue'
import FileUpload from '@/components/tutor/FileUpload.vue'
import VideoPlayer from '@/components/student/VideoPlayer.vue'
import DocumentViewer from '@/components/student/DocumentViewer.vue'
import PptViewer from '@/components/student/PptViewer.vue'
import UiPageHeader from '@/components/ui/UiPageHeader.vue'
import UiCard from '@/components/ui/UiCard.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiTag from '@/components/ui/UiTag.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'
import UiEmptyState from '@/components/ui/UiEmptyState.vue'
import UiErrorState from '@/components/ui/UiErrorState.vue'
import { useAsyncPage } from '@/composables/useAsyncPage'

const route = useRoute()
const router = useRouter()

const chapterNotFound = ref(false)
const chapterDetail = ref<TutorChapterDetail | null>(null)
const courseName = ref('')
const chapters = ref<TutorChapter[]>([])
const activeTab = ref('content')

// 正文编辑
const editContent = ref('')
const originalContent = ref('')
const savingContent = ref(false)
const contentChanged = computed(() => editContent.value !== originalContent.value)

// 文件分组（图片不再作为独立章节文件上传，统一走图文 Markdown 粘贴）
const TYPE_ORDER = ['video', 'document', 'ppt']

/**
 * 媒体类型 → 图标与图标色调。
 * 色调改用语义 token：视频沿用原有红色系（bad）、PPT 沿用橙黄（warn），
 * 文档由 Element 默认蓝改为品牌石墨青 ui-500。
 */
const TYPE_CONFIG: Record<string, { label: string; icon: Component; iconClass: string }> = {
  video: { label: '视频', icon: VideoCamera, iconClass: 'text-bad' },
  document: { label: '文档', icon: Document, iconClass: 'text-ui-500' },
  ppt: { label: 'PPT', icon: Document, iconClass: 'text-warn' }
}

// Vditor 图片上传走 /api/tutor/upload-image（返回 Vditor 期望的 {code,msg,data:{succMap}} 格式）。
// 携带 chapter_id 按章节分目录存储 images/chapters/<chapterId>/，删除章节时可按前缀清理。
const chapterImageUploadUrl = computed(() => {
  const chapterId = chapterDetail.value?.chapter_id
  return chapterId ? `/api/tutor/upload-image?chapter_id=${chapterId}` : '/api/tutor/upload-image'
})

const chapterFiles = computed<ChapterFile[]>(() => chapterDetail.value?.files || [])

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
    icon: TYPE_CONFIG[type]?.icon || Document,
    iconClass: TYPE_CONFIG[type]?.iconClass || 'text-ink-3',
    files: groups[type] || []
  }))
})

// 选中文件预览
const selectedFileId = ref<number | null>(null)
const selectedFile = computed(() => {
  if (selectedFileId.value == null) return null
  return chapterFiles.value.find((f) => f.file_id === selectedFileId.value) || null
})

function selectFile(fileId?: number) {
  if (fileId == null) return
  selectedFileId.value = selectedFileId.value === fileId ? null : fileId
}

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

// 正文保存
async function saveContent() {
  if (!chapterDetail.value) return
  savingContent.value = true
  try {
    await tutorApi.updateChapter(chapterDetail.value.chapter_id, {
      content: editContent.value
    })
    chapterDetail.value.content = editContent.value
    originalContent.value = editContent.value
    ElMessage.success('正文保存成功')
  } catch {
    /* 错误已由拦截器提示 */
  } finally {
    savingContent.value = false
  }
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
  loadChapterDetail()
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
  loadChapterDetail()
}

// 删除文件
async function handleDeleteFile(file: ChapterFile) {
  try {
    await ElMessageBox.confirm(
      `确定要删除文件"${file.file_name}"吗？`,
      '确认删除',
      { confirmButtonText: '确定', cancelButtonText: '取消', type: 'warning' }
    )
    if (file.file_id != null) {
      await tutorApi.deleteFile(file.file_id)
    }
    ElMessage.success('文件删除成功')
    if (selectedFileId.value === file.file_id) {
      selectedFileId.value = null
    }
    await loadChapterDetail()
  } catch {
    /* 取消确认或删除失败；错误已由拦截器提示 */
  }
}

// 数据加载
// 三态收编 useAsyncPage（#401，详情页无分页）：404 归 chapterNotFound 自行渲染，其余异常上抛进 loadError；
// 错误详情由拦截器统一 toast
const { loading, loadError, retrying, retry: handleRetry, run: loadChapterDetail } = useAsyncPage(
  async () => {
    chapterNotFound.value = false
    try {
      const chapterId = Number(route.params.chapterId)
      const res = await tutorApi.getChapterDetail(chapterId)
      chapterDetail.value = res
      editContent.value = res.content || ''
      originalContent.value = res.content || ''
      // 默认 tab：图文优先，否则第一个媒体 tab
      activeTab.value = 'content'
      selectedFileId.value = null
      // 顺便加载课程信息拿课程名 + 章节列表（用于上下章标题）
      await loadCourseInfo()
    } catch (e: unknown) {
      const status = (e as { response?: { status?: number } })?.response?.status
      if (status === 404) {
        chapterNotFound.value = true
      } else {
        throw e
      }
    }
  }
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
  router.push(`/training/tutor/course/${courseId}/chapters`)
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
  selectedFileId.value = null
})

onMounted(() => {
  loadChapterDetail()
})
</script>
