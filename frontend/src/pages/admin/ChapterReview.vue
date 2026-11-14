<template>
  <div class="chapter-review-page">
    <UiPageHeader
      title="章节审核"
      subtitle="待审章节按提交时间排序；预览的是提交时冻结的快照，讲师后续改动不影响在审内容。"
    >
      <template #actions>
        <UiButton :icon="'rotate-cw'" circle aria-label="刷新" @click="load" />
      </template>
    </UiPageHeader>

    <UiFilterBar>
      <template #actions>
        <UiButton variant="success" :disabled="selectedIds.length === 0" @click="batchApprove">
          批量过审（{{ selectedIds.length }}）
        </UiButton>
        <UiButton variant="danger" :disabled="selectedIds.length === 0" @click="openBatchReject">
          批量驳回（{{ selectedIds.length }}）
        </UiButton>
      </template>
    </UiFilterBar>

    <UiAsyncSection
      :loading="loading"
      :error="loadError"
      :retrying="retrying"
      :empty="isEmpty"
      :skeleton="false"
      error-title="待审队列加载失败"
      error-description="网络或服务端异常，可重试"
      @retry="retry"
    >
      <el-table v-loading="loading" :data="list" stripe @selection-change="handleSelection">
        <el-table-column type="selection" width="46" />
        <el-table-column prop="chapter_id" label="ID" width="70" />
        <el-table-column label="章节" min-width="200">
          <template #default="{ row }">
            <div class="chapter-cell">
              <span class="chapter-title">{{ row.chapter_title }}</span>
              <span class="chapter-course text-ink-3">{{ row.course_name }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="文件" width="90">
          <template #default="{ row }">
            <UiTag v-if="row.file_count > 0" tone="info" size="small">{{ row.file_count }} 个</UiTag>
            <span v-else class="text-ink-3 text-sm">—</span>
          </template>
        </el-table-column>
        <el-table-column label="提交时间" width="170">
          <template #default="{ row }">{{ formatLocaleDateTime(row.frozen_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="90" fixed="right" align="center">
          <template #default="{ row }">
            <UiMoreMenu :items="rowActions" trigger="text" @select="(key) => onRowAction(key, row)" />
          </template>
        </el-table-column>
        <template #empty>
          <UiEmptyState title="暂无待审章节" description="讲师提交审核后，章节会出现在这里" size="sm" />
        </template>
      </el-table>
    </UiAsyncSection>

    <UiPagination
      v-if="total > pageSize"
      v-model:current-page="currentPage"
      :page-size="pageSize"
      :total="total"
      align="center"
      class="mt-4"
      @current-change="load"
    />

    <!-- 在审快照预览：正文 + 冻结文件清单（不是 working 草稿） -->
    <UiDialog v-model="snapshotVisible" title="在审快照预览" width="820px">
      <div v-if="snapshot" class="snapshot-body">
        <div class="snapshot-meta">
          <span class="snapshot-title">{{ snapshot.chapter_title }}</span>
          <span class="snapshot-course text-ink-3">{{ snapshot.course_name }}</span>
          <UiTag tone="warning" size="small">提交于 {{ formatLocaleDateTime(snapshot.frozen_at) }}</UiTag>
        </div>
        <div class="snapshot-section">
          <div class="section-label">正文</div>
          <div v-if="!snapshot.content" class="text-ink-3 text-sm">（空正文）</div>
          <!-- 与学员阅读页同一渲染实现（ADR-0046）：审核者看到的必须是学员将看到的样子 -->
          <PublishMarkdown v-else :content="snapshot.content" subset="chapter" class="snapshot-markdown" />
        </div>
        <div class="snapshot-section">
          <div class="section-label">文件挂接清单（{{ snapshot.files.length }}）</div>
          <div v-if="snapshot.files.length === 0" class="text-ink-3 text-sm">本章节无附件</div>
          <ul v-else class="snapshot-files">
            <li v-for="f in snapshot.files" :key="`${f.source}-${f.file_id}`" class="snapshot-file">
              <div class="snapshot-file-head">
                <UiIcon :name="fileCategoryIcon(f.content_type)" class="file-icon" />
                <span class="file-name">{{ f.file_name }}</span>
                <UiTag :tone="describeFileCategory(f.content_type).tone" size="small">
                  {{ describeFileCategory(f.content_type).label }}
                </UiTag>
                <UiTag :tone="describeFileSource(f.source).tone" size="small">
                  {{ describeFileSource(f.source).label }}
                </UiTag>
                <UiButton variant="primary" link size="small" @click="togglePreviewFile(f)">
                  {{ previewFileKey === `${f.source}-${f.file_id}` ? '收起' : '预览' }}
                </UiButton>
              </div>
              <!-- C9「完整内容」：文件本体沿学员端三查看器链路打开，审核者能看清附件里是什么 -->
              <div v-if="previewFileKey === `${f.source}-${f.file_id}`" class="snapshot-file-preview">
                <VideoPlayer v-if="f.content_type === 'video'" :src="f.file_url" />
                <ImageViewer v-else-if="f.content_type === 'image'" :src="f.file_url" :fileName="f.file_name" />
                <PptViewer
                  v-else-if="f.content_type === 'ppt'"
                  :src="f.file_url"
                  :fileName="f.file_name"
                  :chapterId="snapshot.chapter_id"
                />
                <DocumentViewer v-else :src="f.file_url" :fileName="f.file_name" />
              </div>
            </li>
          </ul>
        </div>
      </div>
      <template #footer>
        <UiButton @click="snapshotVisible = false">关闭</UiButton>
        <UiButton variant="danger" @click="rejectFromSnapshot">驳回</UiButton>
        <UiButton variant="success" @click="approveFromSnapshot">通过</UiButton>
      </template>
    </UiDialog>

    <!-- 驳回理由弹窗：单条与批量共用（批量即统一理由） -->
    <UiDialog
      v-model="rejectDialogVisible"
      :title="rejectMode === 'batch' ? '批量驳回（统一理由）' : '驳回章节'"
      width="500px"
    >
      <UiInput
        v-model="rejectReason"
        type="textarea"
        :rows="4"
        placeholder="请填写驳回理由，讲师将看到此说明"
        :maxlength="500"
        show-word-limit
      />
      <template #footer>
        <UiButton @click="closeReject">取消</UiButton>
        <UiButton variant="danger" :loading="rejecting" @click="confirmReject">确认驳回</UiButton>
      </template>
    </UiDialog>
  </div>
</template>

<script setup lang="ts">
// keep-alive 的 include 按**组件名**匹配：名必须与路由名（页面描述符的 name）一致，
// 否则该页不会被缓存（#1620 的锁测试会拦下这种漂移）。
defineOptions({ name: 'ChapterReview' })

import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { adminApi } from '@/api/admin'
import type {
  ChapterReviewQueueItemDTO,
  ChapterReviewSnapshotDTO,
  ChapterFileDTO
} from '@/api/generated/admin'
import { useAdminTable } from '@/composables/useAdminTable'
import { useRejectReasonDialog } from '@/composables/useRejectReasonDialog'
import { useConfirm } from '@/composables/useConfirm'
import { formatLocaleDateTime } from '@/utils/format'
import {
  describeFileCategory,
  describeFileSource,
  fileCategoryIcon
} from '@/utils/chapterFileDisplay'
import UiPageHeader from '@/components/ui/UiPageHeader.vue'
import UiFilterBar from '@/components/ui/UiFilterBar.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiPagination from '@/components/ui/UiPagination.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import UiTag from '@/components/ui/UiTag.vue'
import UiInput from '@/components/ui/UiInput.vue'
import UiIcon from '@/components/ui/UiIcon.vue'
import UiMoreMenu, { type UiMoreMenuItem } from '@/components/ui/UiMoreMenu.vue'
import UiAsyncSection from '@/components/ui/UiAsyncSection.vue'
import UiEmptyState from '@/components/ui/UiEmptyState.vue'
import PublishMarkdown from '@/components/render/PublishMarkdown.vue'
import VideoPlayer from '@/components/student/VideoPlayer.vue'
import DocumentViewer from '@/components/student/DocumentViewer.vue'
import PptViewer from '@/components/student/PptViewer.vue'
import ImageViewer from '@/components/student/ImageViewer.vue'

// 列表状态机（#792 / ADR-0039）：队列无筛选轴，只透传分页。
const { loading, loadError, retrying, list, isEmpty, total, currentPage, pageSize, load, retry } =
  useAdminTable<ChapterReviewQueueItemDTO>({
    fetch: (paging) =>
      adminApi.listPendingChapterReviews({ page: paging.page, page_size: paging.pageSize })
  })

const selectedIds = ref<number[]>([])
const snapshotVisible = ref(false)
const snapshot = ref<ChapterReviewSnapshotDTO | null>(null)
const previewFileKey = ref('')

function handleSelection(rows: ChapterReviewQueueItemDTO[]) {
  selectedIds.value = rows.map((r) => r.chapter_id)
}

/** 行内动作（≥2 个 → UiMoreMenu；删除类动作 divided + tone danger）。 */
const rowActions: UiMoreMenuItem[] = [
  { key: 'preview', label: '预览快照' },
  { key: 'approve', label: '通过' },
  { key: 'reject', label: '驳回', tone: 'danger', divided: true }
]

function onRowAction(key: string, row: ChapterReviewQueueItemDTO) {
  if (key === 'preview') return void previewSnapshot(row)
  if (key === 'approve') return void approveOne(row)
  if (key === 'reject') return void openReject(row)
}

async function previewSnapshot(row: ChapterReviewQueueItemDTO) {
  try {
    snapshot.value = await adminApi.getChapterReviewSnapshot(row.chapter_id)
    previewFileKey.value = ''
    snapshotVisible.value = true
  } catch {
    /* 错误已由拦截器提示 */
  }
}

function togglePreviewFile(f: ChapterFileDTO) {
  const key = `${f.source}-${f.file_id}`
  previewFileKey.value = previewFileKey.value === key ? '' : key
}

/**
 * 单章过审走**单章端点**（#2498 已落，带 409 语义），不走批量接口。
 * 批量接口把逐章失败折进 failures 且整体仍 200，用它做单章会让 UI 报成功而实际没过审。
 */
async function approveOne(row: ChapterReviewQueueItemDTO) {
  try {
    await useConfirm().confirm(`确认通过章节「${row.chapter_title}」？通过后学员端立即更新。`, '通过审核', {
      type: 'success'
    })
  } catch {
    return
  }
  try {
    await adminApi.approveChapterReview(row.chapter_id)
    ElMessage.success('已通过')
    await load()
  } catch {
    /* 错误已由拦截器提示 */
  }
}

async function approveFromSnapshot() {
  if (!snapshot.value) return
  const target = {
    chapter_id: snapshot.value.chapter_id,
    chapter_title: snapshot.value.chapter_title
  } as ChapterReviewQueueItemDTO
  snapshotVisible.value = false
  await approveOne(target)
}

async function batchApprove() {
  const ids = [...selectedIds.value]
  if (ids.length === 0) return
  try {
    await useConfirm().confirm(`确认批量通过 ${ids.length} 个章节？通过后学员端立即更新。`, '批量过审', {
      type: 'success'
    })
  } catch {
    return
  }
  try {
    const res = await adminApi.batchApproveChapterReviews(ids)
    reportBatchResult('过审', res.approved_count, res.failures)
    selectedIds.value = []
    await load()
  } catch {
    /* 错误已由拦截器提示 */
  }
}

/** 批量结果统一提示：成功数 + 失败明细逐条明示（C9「失败明示」，不静默吞掉） */
function reportBatchResult(
  action: string,
  okCount: number,
  failures: { chapter_id: number; reason: string }[]
) {
  if (failures.length === 0) {
    ElMessage.success(`已${action} ${okCount} 个章节`)
    return
  }
  const detail = failures.map((f) => `#${f.chapter_id}：${f.reason}`).join('；')
  ElMessage.warning(`${action}成功 ${okCount} 个，失败 ${failures.length} 个 —— ${detail}`)
}

// 驳回理由弹窗：#795 三域共用状态机；单条/批量/快照三入口共用本弹窗。
const {
  visible: rejectDialogVisible,
  reason: rejectReason,
  submitting: rejecting,
  open: openRejectDialog,
  close: closeRejectDialog,
  submit: submitReject
} = useRejectReasonDialog({
  onSubmit: async (reason) => {
    const ids = rejectMode.value === 'batch' ? [...selectedIds.value] : [rejectTargetId.value]
    const res = await adminApi.batchRejectChapterReviews(ids, reason.trim())
    reportBatchResult('驳回', res.rejected_count, res.failures)
    selectedIds.value = []
    snapshotVisible.value = false
    await load()
  }
})

const rejectMode = ref<'single' | 'batch'>('single')
const rejectTargetId = ref(0)

function openReject(row: ChapterReviewQueueItemDTO) {
  rejectMode.value = 'single'
  rejectTargetId.value = row.chapter_id
  openRejectDialog()
}

function rejectFromSnapshot() {
  if (!snapshot.value) return
  rejectMode.value = 'single'
  rejectTargetId.value = snapshot.value.chapter_id
  openRejectDialog()
}

function openBatchReject() {
  if (selectedIds.value.length === 0) return
  rejectMode.value = 'batch'
  openRejectDialog()
}

function closeReject() {
  closeRejectDialog()
}

function confirmReject() {
  void submitReject()
}

onMounted(load)
</script>

<style scoped>
.chapter-cell {
  display: flex;
  flex-direction: column;
  line-height: 1.35;
}

.chapter-title {
  font-size: var(--text-sm);
  color: var(--color-text-primary);
  font-weight: 500;
}

.chapter-course {
  font-size: var(--text-xs);
}

.snapshot-body {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
}

.snapshot-meta {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  flex-wrap: wrap;
}

.snapshot-title {
  font-size: var(--text-lg);
  font-weight: 600;
  color: var(--color-text-primary);
}

.snapshot-course {
  font-size: var(--text-sm);
}

.snapshot-section {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}

.section-label {
  font-size: var(--text-sm);
  font-weight: 600;
  color: var(--color-text-secondary);
}

.snapshot-markdown {
  max-height: 340px;
  overflow-y: auto;
  padding: var(--space-3);
  border-radius: var(--radius-ctl);
  background: var(--color-surface-sunken);
}

.snapshot-files {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  margin: 0;
  padding: 0;
  list-style: none;
}

.snapshot-file {
  padding: var(--space-2) var(--space-3);
  border-radius: var(--radius-ctl);
  background: var(--color-surface-sunken);
}

.snapshot-file-head {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  flex-wrap: wrap;
}

.snapshot-file-preview {
  margin-top: var(--space-3);
}

.file-icon {
  color: var(--color-text-tertiary);
}

.file-name {
  font-size: var(--text-sm);
  color: var(--color-text-primary);
  word-break: break-all;
}
</style>
