<template>
  <div>
    <UiPageHeader
      :title="pageTitle"
      :subtitle="chapters.length > 0 ? `共 ${chapters.length} 个章节` : undefined"
      back
      @back="goBack"
    >
      <template #meta>
        <p class="text-sm text-ink-3">点击章节进入编辑，可修改正文、上传与预览多种类型内容</p>
      </template>
    </UiPageHeader>

    <!-- 认领条（#2596 D3）：已认领→共认名单+退出；未认领→提示条+认领按钮（写操作由后端 403 兜底）；
         名单拉失败→显式说「状态未知」并给重试，而不是默默隐藏整条（那样用户会以为这页没有认领这回事，
         点写按钮才 403）。读面不受认领限制，故未认领也照常能看到章节列表——只是改不动。 -->
    <div
      v-if="claimLoaded || claimFailed"
      class="mb-4 flex flex-wrap items-center justify-between gap-3 rounded-md border px-4 py-3"
      :class="claimedByMe ? 'border-line bg-ui-soft' : 'border-warn/40 bg-warn-soft'"
    >
      <div class="min-w-0 text-sm">
        <template v-if="claimFailed">
          <span class="font-medium text-ink">认领状态加载失败</span>
          <span class="text-ink-3">；不影响浏览章节，写操作仍会按后端校验拒绝</span>
        </template>
        <template v-else-if="claimedByMe">
          <span class="font-medium text-ink">你已认领本课程</span>
          <span v-if="otherClaimants.length" class="text-ink-3">
            ；同事：{{ otherClaimants.map((c) => c.tutor_name).join('、') }} 也在编
          </span>
          <span v-else class="text-ink-3">；暂无其他讲师共认</span>
        </template>
        <template v-else>
          <span class="font-medium text-ink">认领本课程后才能编辑章节</span>
          <span class="text-ink-3">；现在可以先浏览内容再决定</span>
        </template>
      </div>
      <div class="flex shrink-0 items-center gap-2">
        <UiButton v-if="claimFailed" size="small" :loading="retryingClaimants" @click="retryClaimants">
          重试
        </UiButton>
        <UiButton
          v-else-if="!claimedByMe"
          size="small"
          variant="primary"
          :loading="claiming"
          @click="handleClaim"
        >
          认领课程
        </UiButton>
        <UiButton v-else size="small" :loading="releasing" @click="handleRelease">
          退出认领
        </UiButton>
      </div>
    </div>

    <UiAsyncSection
      :error="loadError"
      :loading="loading"
      :empty="isEmpty"
      :retrying="retrying"
      error-title="章节加载失败"
      error-description="未能获取该课程的章节列表，请检查网络后重试。"
      @retry="handleRetry"
    >
      <template #skeleton>
        <UiSkeleton variant="list" :count="5"  />
      </template>

      <div class="flex flex-col gap-3">
        <router-link
          v-for="(chapter, index) in chapters"
          :key="chapter.chapter_id"
          :to="{ name: 'TutorChapterEdit', params: { courseId: route.params.id, chapterId: chapter.chapter_id } }"
          class="block no-underline"
        >
          <UiCard variant="interactive" class="group flex items-center gap-4">
            <span
              class="flex h-9 w-9 shrink-0 items-center justify-center rounded-pill bg-ui-soft text-sm font-semibold text-ui-soft-ink"
            >
              {{ String(index + 1).padStart(2, '0') }}
            </span>

            <div class="min-w-0 flex-1">
              <p class="truncate text-base font-medium text-ink">{{ chapter.title }}</p>
              <div class="mt-2 flex flex-wrap items-center gap-3">
                <UiTag :tone="contentTypeTone(chapter.content_type || '')">
                  {{ contentTypeLabel(chapter.content_type || '') }}
                </UiTag>
                <span v-if="chapter.duration" class="inline-flex items-center gap-1 text-xs text-ink-3">
                  <UiIcon name="timer" /> {{ chapter.duration }}分钟
                </span>
                <span v-if="getFileCount(chapter) > 0" class="inline-flex items-center gap-1 text-xs text-ink-3">
                  <UiIcon name="file-text" /> {{ getFileCount(chapter) }} 个文件
                </span>
                <span v-if="chapter.content" class="inline-flex items-center gap-1 text-xs text-ok">
                  <UiIcon name="square-pen" /> 含正文
                </span>
              </div>
            </div>

            <UiIcon name="arrow-right" class="shrink-0 text-ink-3 transition-colors duration-150 group-hover:underline max-sm:hidden" />
          </UiCard>
        </router-link>
      </div>
      <template #empty>
        <UiEmptyState
      title="暂无章节"
      description="该课程还没有任何章节，可返回课程列表继续其他操作。"
      action-text="返回课程列表"
      @action="goBack"
     />
      </template>
    </UiAsyncSection>
  </div>
</template>

<script setup lang="ts">
import UiIcon from '@/components/ui/UiIcon.vue'
import { computed, ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { tutorApi, type CourseClaimant, type TutorChapter, type TutorCourse } from '@/api/tutor'
import UiCard from '@/components/ui/UiCard.vue'
import UiTag from '@/components/ui/UiTag.vue'
import UiPageHeader from '@/components/ui/UiPageHeader.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'
import UiAsyncSection from '@/components/ui/UiAsyncSection.vue'
import UiEmptyState from '@/components/ui/UiEmptyState.vue'
import UiButton from '@/components/ui/UiButton.vue'
import { useConfirm } from '@/composables/useConfirm'
import { useAsyncPage } from '@/composables/useAsyncPage'
import { ElMessage } from 'element-plus'

type ChapterRow = TutorChapter & { content_type?: string; files?: unknown[] }
type ContentTone = 'brand' | 'success' | 'warning' | 'danger' | 'neutral'

const route = useRoute()
const router = useRouter()

const courseInfo = ref<TutorCourse | null>(null)
const chapters = ref<ChapterRow[]>([])

// 三态收编 useAsyncPage（#401）：错误详情由拦截器统一 toast，retry 防重入由 composable 提供
const { loading, loadError, retrying, isEmpty, retry: handleRetry, run: loadChapters } = useAsyncPage(
  async () => {
    const courseId = Number(route.params.id)
    const res = await tutorApi.getCourseChapters(courseId)
    return res
  },
  {
    apply: (res) => {
      courseInfo.value = res.course ?? null
      chapters.value = res.chapters || []
    }, itemsRef: chapters }
)

const pageTitle = computed(() =>
  courseInfo.value?.name ? `${courseInfo.value.name} · 章节管理` : '章节管理'
)

/** 内容类型 → 语义色调。视频不再用 danger（红色=告警语义），改用 brand 石墨青。 */
const CONTENT_TONE: Record<string, ContentTone> = {
  text: 'neutral',
  document: 'brand',
  ppt: 'warning',
  video: 'brand',
  image: 'success'
}

const CONTENT_LABEL: Record<string, string> = {
  text: '图文',
  document: '文档',
  ppt: 'PPT',
  video: '视频',
  image: '图片'
}

function contentTypeTone(contentType: string): ContentTone {
  return CONTENT_TONE[contentType] || 'neutral'
}

function contentTypeLabel(contentType: string): string {
  return CONTENT_LABEL[contentType] || contentType || '未设置'
}

function getFileCount(chapter: { files?: unknown[] }): number {
  return (chapter.files || []).length
}

function goBack() {
  router.push({ name: 'TutorCourses' })
}

// ===== 认领条（#2596 D3/D5）=====
// 「我在不在名单里」由服务端回答（claimed_by_me）：前端拿 user_id 自己比对会多出一条
// 「我是谁」的判定链，而这本来是服务端一眼可知的事实。
const claimants = ref<CourseClaimant[]>([])
const claimedByMe = ref(false)
const claimLoaded = ref(false)
const claimFailed = ref(false)
const claiming = ref(false)
const releasing = ref(false)
const retryingClaimants = ref(false)

/** 共认名单里除我之外的人（服务端逐行标 is_me，前端不自己比 id）。 */
const otherClaimants = computed(() => claimants.value.filter((c) => !c.is_me))

async function loadClaimants() {
  try {
    const res = await tutorApi.getCourseClaimants(Number(route.params.id))
    claimants.value = res.claimants || []
    claimedByMe.value = res.claimed_by_me
    claimLoaded.value = true
    claimFailed.value = false
  } catch {
    // 名单拉不到不阻断读章节（拦截器已 toast），但**不能静默藏起来**——
    // 藏了用户就以为这页没有认领这回事，点写按钮才吃 403。显式落到「状态未知 + 重试」。
    claimFailed.value = true
  }
}

async function handleClaim() {
  if (claiming.value) return
  claiming.value = true
  try {
    await tutorApi.claimCourse(Number(route.params.id))
    ElMessage.success('已认领，可以开始编辑章节')
    await loadClaimants()
  } catch {
    // 409 等由拦截器统一 toast；重取名单把状态拉回真相
    await loadClaimants()
  } finally {
    claiming.value = false
  }
}

/** 共认名单里除我之外的人（我未认领时就是全部）。 */
async function handleRelease() {
  if (releasing.value) return
  try {
    // 退出不回溯是这个动作最容易被误解的地方，确认框必须写明（B6）
    await useConfirm().confirm(
      '退出后你不能再编辑本课程；已写章节的归属与在审内容不受影响。',
      '退出认领'
    )
  } catch {
    return // 用户取消
  }
  releasing.value = true
  try {
    await tutorApi.releaseCourseClaim(Number(route.params.id))
    ElMessage.success('已退出认领')
    await loadClaimants()
  } catch {
    await loadClaimants()
  } finally {
    releasing.value = false
  }
}

onMounted(() => {
  loadChapters()
  loadClaimants()
})

// 认领条的重试入口：名单是独立于章节列表的一次请求，它的失败不该借用页面的三态骨架
// （章节列表本身的错误由 useAsyncPage 管）。故单独一个「重拉名单」动作挂在提示条上。
async function retryClaimants() {
  if (retryingClaimants.value) return
  retryingClaimants.value = true
  try {
    await loadClaimants()
  } finally {
    retryingClaimants.value = false
  }
}
</script>
