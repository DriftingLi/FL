import { computed, onScopeDispose, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { tutorApi, type TutorChapterDetail } from '@/api/tutor'
import type { ChapterFile } from '@/api/course'

export type ChapterClaimState = 'unknown' | 'claimed' | 'unclaimed'

export interface ChapterReviewSnapshot {
  content: string
  files: ChapterFile[]
}

function cloneFiles(files: ChapterFile[]): ChapterFile[] {
  return files.map(file => ({ ...file }))
}

function cloneReviewSnapshot(detail: TutorChapterDetail): ChapterReviewSnapshot | null {
  return detail.review_snapshot
    ? { content: detail.review_snapshot.content, files: cloneFiles(detail.review_snapshot.files) }
    : null
}

/** 章节编辑页的局部会话：草稿、已保存版本、在审快照、文件事实与可写状态同生共死。 */
export function useChapterEditingSession() {
  const chapterDetail = ref<TutorChapterDetail | null>(null)
  const editContent = ref('')
  const savedContent = ref('')
  const chapterFiles = ref<ChapterFile[]>([])
  const reviewSnapshot = ref<ChapterReviewSnapshot | null>(null)
  const activeTab = ref('content')
  const selectedFileKey = ref<string | null>(null)

  const claimState = ref<ChapterClaimState>('unknown')
  const claiming = ref(false)
  const canWrite = computed(() => claimState.value === 'claimed')
  const contentChanged = computed(() => editContent.value !== savedContent.value)
  const savingContent = ref(false)
  const submittingReview = ref(false)

  function fileKey(file: ChapterFile): string {
    return `${file.source || 'chapter_file'}:${file.file_id}`
  }

  const selectedFile = computed(() => {
    if (selectedFileKey.value == null) return null
    return chapterFiles.value.find(file => fileKey(file) === selectedFileKey.value) || null
  })

  function selectFile(file?: ChapterFile) {
    if (!file) return
    const key = fileKey(file)
    selectedFileKey.value = selectedFileKey.value === key ? null : key
  }

  /** 初次装载/切章：建立一份全新的工作副本，旧章节草稿与快照不得带过来。 */
  function loadChapter(detail: TutorChapterDetail) {
    chapterDetail.value = detail
    editContent.value = detail.content || ''
    savedContent.value = detail.content || ''
    chapterFiles.value = cloneFiles(detail.files || [])
    reviewSnapshot.value = cloneReviewSnapshot(detail)
    activeTab.value = 'content'
    selectedFileKey.value = null
  }

  /**
   * 文件操作后的服务端事实刷新：保留工作副本与已保存基线。
   * 没有本地草稿时采纳服务端正文，使其成为可保存的新工作副本；已保存基线仍只由保存动作推进。
   */
  async function refreshFiles() {
    const chapterId = chapterDetail.value?.chapter_id
    if (!chapterId) return
    try {
      const detail = await tutorApi.getChapterDetail(chapterId)
      if (chapterDetail.value?.chapter_id !== chapterId) return
      const hadLocalDraft = contentChanged.value
      chapterDetail.value = detail
      chapterFiles.value = cloneFiles(detail.files || [])
      reviewSnapshot.value = cloneReviewSnapshot(detail)
      if (!hadLocalDraft) editContent.value = detail.content || ''
      if (selectedFileKey.value && !selectedFile.value) selectedFileKey.value = null
    } catch (error) {
      console.error('[chapter editing session] 文件事实刷新失败，保留当前工作副本。', error)
    }
  }

  async function loadClaimState(courseId: number) {
    if (!courseId) {
      claimState.value = 'unclaimed'
      return
    }
    try {
      const result = await tutorApi.getCourseClaimants(courseId)
      claimState.value = result.claimed_by_me ? 'claimed' : 'unclaimed'
    } catch {
      claimState.value = 'unknown'
    }
  }

  async function claimCourse(courseId: number) {
    if (claiming.value) return
    claiming.value = true
    try {
      await tutorApi.claimCourse(courseId)
      ElMessage.success('已认领，可以开始编辑章节')
      claimState.value = 'claimed'
    } catch {
      await loadClaimState(courseId)
    } finally {
      claiming.value = false
    }
  }

  async function saveContent() {
    const detail = chapterDetail.value
    if (!detail || !canWrite.value || !contentChanged.value || savingContent.value) return
    const submittedContent = editContent.value
    savingContent.value = true
    try {
      const updated = await tutorApi.updateChapter(detail.chapter_id, { content: submittedContent })
      savedContent.value = submittedContent
      detail.content = submittedContent
      if (updated.review_status) detail.review_status = updated.review_status
      if (detail.review_status !== 'pending') reviewSnapshot.value = null
      ElMessage.success('正文保存成功')
    } catch {
      // 错误已由拦截器提示
    } finally {
      savingContent.value = false
    }
  }

  /** 提审是独立动作：只冻结已保存正文与当前文件清单，工作副本随后可继续编辑。 */
  async function submitReview(confirmSubmission: () => Promise<unknown>) {
    const detail = chapterDetail.value
    if (!detail || !canWrite.value || submittingReview.value || detail.review_status === 'pending') return
    if (contentChanged.value) {
      ElMessage.warning('请先保存正文后再提交审核')
      return
    }

    try {
      await confirmSubmission()
    } catch {
      return
    }
    if (chapterDetail.value?.chapter_id !== detail.chapter_id || !canWrite.value) return
    if (contentChanged.value || chapterDetail.value.review_status === 'pending') return

    submittingReview.value = true
    try {
      const updated = await tutorApi.submitChapterReview(detail.chapter_id)
      if (chapterDetail.value?.chapter_id === detail.chapter_id) {
        chapterDetail.value.review_status = updated.review_status ?? 'pending'
        chapterDetail.value.reject_reason = updated.reject_reason ?? ''
        reviewSnapshot.value = null
        try {
          const refreshed = await tutorApi.getChapterDetail(detail.chapter_id)
          if (chapterDetail.value?.chapter_id === detail.chapter_id) {
            chapterDetail.value.review_status = refreshed.review_status ?? 'pending'
            chapterDetail.value.reject_reason = refreshed.reject_reason ?? ''
            reviewSnapshot.value = cloneReviewSnapshot(refreshed)
          }
        } catch (error) {
          if (chapterDetail.value?.chapter_id === detail.chapter_id) {
            chapterDetail.value.review_status = 'pending'
            reviewSnapshot.value = null
            console.error('[chapter editing session] 提审成功但在审快照刷新失败。', error)
            ElMessage.warning('已提交审核，但在审快照刷新失败，请刷新页面重试')
          }
        }
      }
      ElMessage.success('已提交审核')
    } catch {
      // 错误已由拦截器提示（重复提交 400 / 章节不存在 404）
    } finally {
      submittingReview.value = false
    }
  }

  onScopeDispose(() => {
    chapterDetail.value = null
    editContent.value = ''
    savedContent.value = ''
    chapterFiles.value = []
    reviewSnapshot.value = null
    selectedFileKey.value = null
    claimState.value = 'unknown'
  })

  return {
    chapterDetail,
    editContent,
    savedContent,
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
    fileKey,
    selectFile,
    loadChapter,
    refreshFiles,
    loadClaimState,
    claimCourse,
    saveContent,
    submitReview
  }
}
