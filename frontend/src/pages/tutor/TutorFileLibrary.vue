<template>
  <div>
    <UiPageHeader title="文件库" subtitle="上传的录课视频与课件在这里管理">
      <template #meta>
        <p class="text-sm text-ink-3">支持单文件最大 2GB；大文件自动分片上传，中断后重新选择即可</p>
      </template>
      <template #actions>
        <UiUpload
          class="[&_.el-upload]:flex [&_.el-upload]:cursor-pointer"
          :show-file-list="false"
          :accept="TUTOR_FILE_ACCEPT"
          :http-request="handleUpload"
        >
          <UiButton variant="primary" :loading="uploading">上传文件</UiButton>
        </UiUpload>
      </template>
    </UiPageHeader>

    <UiCard v-if="uploading" class="mb-4">
      <div class="flex items-center gap-3">
        <p class="min-w-0 flex-1 truncate text-sm text-ink">{{ uploadingName }}</p>
        <span class="text-sm tabular-nums text-ink-3">{{ uploadPercent }}%</span>
      </div>
      <UiProgress class="mt-2" :value="uploadPercent" size="sm" />
    </UiCard>

    <UiFilterBar>
      <template #filters>
        <UiSelect
          v-model="typeFilter"
          :options="TYPE_OPTIONS"
          placeholder="全部类型"
          clearable
          class="w-32"
        />
        <UiSelect
          v-model="attachFilter"
          :options="ATTACH_OPTIONS"
          placeholder="全部挂接状态"
          clearable
          class="w-36"
        />
        <el-date-picker
          v-model="dateRange"
          type="daterange"
          range-separator="至"
          start-placeholder="开始日期"
          end-placeholder="结束日期"
          value-format="YYYY-MM-DD"
          class="!w-64"
        />
      </template>
      <template #actions>
        <UiButton variant="text" @click="resetFilters">重置</UiButton>
      </template>
    </UiFilterBar>

    <UiAsyncSection
      :error="loadError"
      :loading="loading"
      :empty="isEmpty"
      :retrying="retrying"
      error-title="文件库加载失败"
      error-description="未能获取文件列表，请检查网络后重试。"
      @retry="handleRetry"
    >
      <template #skeleton>
        <UiSkeleton variant="list" :count="3" />
      </template>

      <div class="flex flex-col gap-3">
        <UiCard
          v-for="file in files"
          :key="file.file_id"
          class="flex cursor-pointer items-center gap-4"
          @click="openPreview(file)"
        >
          <UiIcon name="file-text" class="shrink-0 text-ink-3" />
          <div class="min-w-0 flex-1">
            <p class="truncate text-base font-medium text-ink">{{ file.file_name }}</p>
            <div class="mt-1 flex flex-wrap items-center gap-3 text-xs text-ink-3">
              <UiTag :tone="describeCategory(file.content_type).tone">
                {{ describeCategory(file.content_type).label }}
              </UiTag>
              <UiTag :tone="describeAttach(file).tone">
                {{ describeAttach(file).label }}
              </UiTag>
              <span class="tabular-nums">{{ formatSize(file.file_size) }}</span>
            </div>
          </div>
          <UiMoreMenu :items="menuItems(file)" @select="(key) => onMenuAction(key, file)" />
        </UiCard>
      </div>

      <template #empty>
        <UiEmptyState
          title="文件库是空的"
          description="上传录课视频或课件后，可在此管理并挂接到章节。"
        />
      </template>
    </UiAsyncSection>

    <!-- 预览：沿既有三查看器链路（VideoPlayer / DocumentViewer / PptViewer / ImageViewer） -->
    <UiDialog v-model="previewVisible" :title="previewing?.file_name || '预览'" width="760px">
      <div v-if="previewing" class="min-h-[200px]">
        <VideoPlayer v-if="previewing.content_type === 'video'" :src="previewing.file_url" />
        <ImageViewer
          v-else-if="previewing.content_type === 'image'"
          :src="previewing.file_url"
          :file-name="previewing.file_name"
        />
        <PptViewer
          v-else-if="previewing.content_type === 'ppt'"
          :src="previewing.file_url"
          :file-name="previewing.file_name"
          :chapter-id="previewing.chapter_id"
        />
        <DocumentViewer
          v-else
          :src="previewing.file_url"
          :file-name="previewing.file_name"
        />
      </div>
    </UiDialog>

    <!-- 挂接：选课程 → 选章节（单挂，一个文件同一时间只挂一章） -->
    <UiDialog
      v-model="attachVisible"
      title="挂接到章节"
      width="480px"
      confirm-text="挂接"
      :confirm-loading="attaching"
      @confirm="confirmAttach"
    >
      <el-form label-width="80px">
        <el-form-item label="课程">
          <UiSelect
            v-model="attachCourseId"
            :options="courseOptions"
            placeholder="选择课程"
            filterable
            class="w-full"
          />
        </el-form-item>
        <el-form-item label="章节">
          <UiSelect
            v-model="attachChapterId"
            :options="chapterOptions"
            placeholder="选择章节"
            :disabled="!attachCourseId"
            filterable
            class="w-full"
          />
        </el-form-item>
      </el-form>
    </UiDialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiCard from '@/components/ui/UiCard.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import UiEmptyState from '@/components/ui/UiEmptyState.vue'
import UiFilterBar from '@/components/ui/UiFilterBar.vue'
import UiIcon from '@/components/ui/UiIcon.vue'
import UiMoreMenu, { type UiMoreMenuItem } from '@/components/ui/UiMoreMenu.vue'
import UiPageHeader from '@/components/ui/UiPageHeader.vue'
import UiProgress from '@/components/ui/UiProgress.vue'
import UiSelect, { type UiSelectOption } from '@/components/ui/UiSelect.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'
import UiTag from '@/components/ui/UiTag.vue'
import UiUpload from '@/components/ui/UiUpload.vue'
import UiAsyncSection from '@/components/ui/UiAsyncSection.vue'
import DocumentViewer from '@/components/student/DocumentViewer.vue'
import ImageViewer from '@/components/student/ImageViewer.vue'
import PptViewer from '@/components/student/PptViewer.vue'
import VideoPlayer from '@/components/student/VideoPlayer.vue'
import { useAsyncPage } from '@/composables/useAsyncPage'
import { useConfirm } from '@/composables/useConfirm'
import { tutorApi, type TutorChapter, type TutorCourse } from '@/api/tutor'
import { tutorFileApi, uploadFileInChunks, TUTOR_FILE_ACCEPT, type FileDTO } from '@/api/tutorfile'

const uploading = ref(false)
const uploadingName = ref('')
const uploadPercent = ref(0)

const files = ref<FileDTO[]>([])

// 筛选（spec B7）：类型 / 挂接状态 / 创建时间区间。筛选变化唯一入口 filterDeps。
const typeFilter = ref<string | number | null>(null)
const attachFilter = ref<string | number | null>(null)
const dateRange = ref<[string, string] | null>(null)

const TYPE_OPTIONS: UiSelectOption[] = [
  { label: '视频', value: 'video' },
  { label: '文档', value: 'document' },
  { label: '幻灯片', value: 'ppt' },
  { label: '图片', value: 'image' }
]
const ATTACH_OPTIONS: UiSelectOption[] = [
  { label: '已挂接', value: 'attached' },
  { label: '未挂接', value: 'unattached' }
]

const {
  loading,
  loadError,
  retrying,
  isEmpty,
  retry: handleRetry,
  run: loadFiles
} = useAsyncPage(
  async () => {
    const filter: Parameters<typeof tutorFileApi.listFiles>[0] = {}
    if (typeFilter.value) filter.content_type = String(typeFilter.value)
    if (attachFilter.value) filter.attached = attachFilter.value === 'attached'
    if (dateRange.value) {
      filter.created_from = dateRange.value[0]
      filter.created_to = dateRange.value[1]
    }
    return (await tutorFileApi.listFiles(filter)).files
  },
  {
    apply: (list) => {
      files.value = list
    },
    itemsRef: files,
    filterDeps: [typeFilter, attachFilter, dateRange]
  }
)

function resetFilters() {
  typeFilter.value = null
  attachFilter.value = null
  dateRange.value = null
}

onMounted(() => {
  loadFiles()
})

/** 内容分类 → 语义色调（状态词表口径：单一 descriptor 派生 label 与 tone）。 */
function describeCategory(t: string): { label: string; tone: 'brand' | 'success' | 'info' | 'neutral' } {
  const map: Record<string, { label: string; tone: 'brand' | 'success' | 'info' | 'neutral' }> = {
    video: { label: '视频', tone: 'brand' },
    document: { label: '文档', tone: 'info' },
    ppt: { label: '幻灯片', tone: 'success' },
    image: { label: '图片', tone: 'neutral' }
  }
  return map[t] ?? { label: t, tone: 'neutral' }
}

/** 挂接状态 → descriptor（状态词表口径：单一 descriptor 派生 label 与 tone）。 */
function describeAttach(file: FileDTO): { label: string; tone: 'brand' | 'neutral' } {
  if (!file.chapter_id) return { label: '未挂接', tone: 'neutral' }
  return { label: `已挂：${file.chapter_title || `章节 #${file.chapter_id}`}`, tone: 'brand' }
}

function formatSize(size: number) {
  if (size >= 1 << 30) return (size / (1 << 30)).toFixed(2) + ' GB'
  if (size >= 1 << 20) return (size / (1 << 20)).toFixed(1) + ' MB'
  return Math.max(1, Math.round(size / 1024)) + ' KB'
}

/** UiUpload 的 http-request 槽：接管传输（分片上传），不再走 el-upload 自带单发。 */
async function handleUpload({ file }: { file: File }) {
  uploading.value = true
  uploadingName.value = file.name
  uploadPercent.value = 0
  try {
    await uploadFileInChunks(file, (p) => (uploadPercent.value = p))
    await loadFiles()
  } catch {
    // 错误已由请求层统一提示；此处只收尾 UI
  } finally {
    uploading.value = false
  }
}

// 行动作收进溢出菜单（低频、破坏性类）；预览走整行点击（对象即入口）。
function menuItems(file: FileDTO): UiMoreMenuItem[] {
  const items: UiMoreMenuItem[] = [{ key: 'rename', label: '改名' }]
  if (file.chapter_id) {
    items.push({ key: 'detach', label: '取消挂接（回文件库）' })
  } else {
    items.push({ key: 'attach', label: '挂接到章节' })
  }
  items.push({ key: 'delete', label: '删除', tone: 'danger', divided: true })
  return items
}

async function onMenuAction(key: string, file: FileDTO) {
  switch (key) {
    case 'rename':
      await onRename(file)
      break
    case 'attach':
      openAttach(file)
      break
    case 'detach':
      await onDetach(file)
      break
    case 'delete':
      await onDelete(file)
      break
  }
}

// 改名走输入式确认（登记形态 useConfirm().prompt）
async function onRename(file: FileDTO) {
  let value: string
  try {
    const res = await useConfirm().prompt('输入新的文件名：', '改名', {
      inputValue: file.file_name,
      inputValidator: (v: string) => (v.trim() ? true : '文件名不能为空')
    })
    value = res.value
  } catch {
    return
  }
  await tutorFileApi.renameFile(file.file_id, value.trim())
  await loadFiles()
}

async function onDetach(file: FileDTO) {
  try {
    await useConfirm().confirm(`取消挂接「${file.file_name}」？文件将回到文件库未挂状态。`, '取消挂接')
  } catch {
    return
  }
  await tutorFileApi.detachFile(file.file_id)
  await loadFiles()
}

async function onDelete(file: FileDTO) {
  try {
    await useConfirm().confirmDanger(`确认删除「${file.file_name}」？删除后无法恢复。`, '删除文件')
  } catch {
    return
  }
  await tutorFileApi.deleteFile(file.file_id)
  await loadFiles()
}

// 预览（spec B7：沿既有三查看器链路）
const previewVisible = ref(false)
const previewing = ref<FileDTO | null>(null)

function openPreview(file: FileDTO) {
  previewing.value = file
  previewVisible.value = true
}

// 挂接：课程 → 章节级联（单挂；换挂在章节编辑面做，spec B6）
const attachVisible = ref(false)
const attaching = ref(false)
const attachFileId = ref<number | null>(null)
const attachCourseId = ref<string | number | null>(null)
const attachChapterId = ref<string | number | null>(null)
const courses = ref<TutorCourse[]>([])
const chapters = ref<TutorChapter[]>([])

const courseOptions = computed<UiSelectOption[]>(() =>
  courses.value.map((c) => ({ label: c.name, value: c.course_id }))
)
const chapterOptions = computed<UiSelectOption[]>(() =>
  chapters.value.map((c) => ({ label: c.title, value: c.chapter_id }))
)

watch(attachCourseId, async (cid) => {
  attachChapterId.value = null
  chapters.value = []
  if (cid == null) return
  const res = await tutorApi.getCourseChapters(Number(cid))
  chapters.value = res.chapters || []
})

async function openAttach(file: FileDTO) {
  attachFileId.value = file.file_id
  attachCourseId.value = null
  attachChapterId.value = null
  attachVisible.value = true
  if (courses.value.length === 0) {
    const res = await tutorApi.getCourses({})
    courses.value = res.courses || []
  }
}

async function confirmAttach() {
  if (attachFileId.value == null || attachChapterId.value == null) return
  attaching.value = true
  try {
    await tutorFileApi.attachFile(attachFileId.value, Number(attachChapterId.value))
    attachVisible.value = false
    await loadFiles()
  } catch {
    // 错误已由请求层统一提示
  } finally {
    attaching.value = false
  }
}
</script>
