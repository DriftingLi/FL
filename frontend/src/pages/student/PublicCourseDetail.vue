<template>
  <main class="mx-auto flex w-full max-w-5xl flex-col gap-5 px-4 py-6">
    <nav class="flex items-center justify-between gap-2" aria-label="课程导航">
      <RouterLink :to="href('CourseList')" class="text-sm font-medium text-ui-soft-ink">返回课程中心</RouterLink>
      <ThemeToggle variant="ghost" />
    </nav>
    <UiAsyncSection :loading="loading" :error="loadError" :empty="isEmpty" :retrying="retrying"
      error-title="课程加载失败" @retry="retry">
      <template #empty>
        <UiEmptyState title="课程不存在" description="该课程可能已下架。">
          <RouterLink :to="href('CourseList')" class="text-ui-soft-ink">浏览其他课程</RouterLink>
        </UiEmptyState>
      </template>
      <template v-if="course">
        <UiCard>
          <div class="grid gap-5 sm:grid-cols-[minmax(0,2fr)_minmax(0,3fr)]">
            <img v-if="course.cover_image" :src="course.cover_image" alt="" class="aspect-video w-full rounded-card object-cover" />
            <div class="min-w-0 space-y-4" :class="{ 'sm:col-span-2': !course.cover_image }">
              <div class="flex flex-wrap gap-2">
                <UiTag>{{ course.category.name }}</UiTag>
                <UiTag v-if="course.points_price" tone="warning">{{ course.points_price }} 积分解锁</UiTag>
              </div>
              <h1 class="m-0 break-words text-2xl font-semibold text-ink">{{ course.name }}</h1>
              <p v-if="course.description" class="whitespace-pre-wrap break-words text-ink-2">{{ course.description }}</p>
              <p v-if="study?.is_enrolled" class="text-sm text-ink-3">已学 {{ Math.round(study.progress) }}%</p>
              <div class="flex flex-wrap gap-2">
                <UiButton v-if="course.chapters.length || locked" variant="primary" :loading="redeeming" @click="begin()">
                  {{ actionLabel }}
                </UiButton>
                <UiButton v-if="isStudent" variant="ghost" :icon="favorited ? 'star-filled' : 'star'" @click="toggleFavorite">
                  {{ favorited ? '取消收藏' : '收藏课程' }}
                </UiButton>
              </div>
            </div>
          </div>
        </UiCard>
        <UiCard>
          <h2 class="m-0 text-lg font-semibold text-ink">章节目录</h2>
          <UiEmptyState v-if="!course.chapters.length" size="sm" description="暂无已发布章节" />
          <ol v-else class="m-0 list-none divide-y divide-line p-0">
            <li v-for="(chapter, index) in course.chapters" :key="chapter.chapter_id" class="flex items-center justify-between gap-4 py-2">
              <UiButton variant="ghost" @click="begin(chapter.chapter_id)">{{ index + 1 }}. {{ chapter.title }}</UiButton>
              <span v-if="chapter.duration > 0" class="shrink-0 text-sm text-ink-3">{{ chapter.duration }} 分钟</span>
            </li>
          </ol>
        </UiCard>
      </template>
    </UiAsyncSection>
  </main>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { courseApi, type CourseDetailDTO, type PublicCourseDetailDTO } from '@/api/course'
import { pointsApi } from '@/api/points'
import ThemeToggle from '@/components/ui/ThemeToggle.vue'
import UiAsyncSection from '@/components/ui/UiAsyncSection.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiCard from '@/components/ui/UiCard.vue'
import UiEmptyState from '@/components/ui/UiEmptyState.vue'
import UiTag from '@/components/ui/UiTag.vue'
import { useAsyncPage } from '@/composables/useAsyncPage'
import { useConfirm } from '@/composables/useConfirm'
import { useFavorite } from '@/composables/useFavorite'
import { href } from '@/config/pages'
import { useAuthStore } from '@/stores/auth'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const confirm = useConfirm()
const courseId = computed(() => Number(route.params.courseId))
const isStudent = computed(() => auth.isLoggedIn && auth.userInfo?.role === 'hrwai_user')
const course = ref<PublicCourseDetailDTO | null>(null)
const study = ref<CourseDetailDTO | null>(null)
const redeeming = ref(false)

const { loading, loadError, isEmpty, retrying, retry, run } = useAsyncPage(
  async () => {
    const id = courseId.value
    const publicDetail = await courseApi.getPublicCourseDetail(id)
    const studentDetail = isStudent.value ? await courseApi.getCourseDetail(id) : null
    return { publicDetail, studentDetail }
  },
  {
    credentialScoped: false,
    itemsRef: course,
    apply: ({ publicDetail, studentDetail }) => {
      course.value = publicDetail
      study.value = studentDetail
    }
  }
)
watch([courseId, isStudent], () => {
  course.value = null
  study.value = null
  void run()
}, { immediate: true })
const { favorited, load: loadFavorite, toggle: toggleFavorite } = useFavorite('course', () => course.value?.course_id)
watch(() => course.value?.course_id, (id) => {
  if (id && isStudent.value) {
    void courseApi.recordCourseBrowse(id).catch(() => undefined)
    void loadFavorite()
  }
})

const locked = computed(() => isStudent.value && Boolean(course.value?.points_price) && study.value?.course_info.entitled !== true)
const resumeChapter = computed(() => course.value?.chapters.find(chapter => chapter.chapter_id === study.value?.last_chapter_id))
const actionLabel = computed(() => {
  if (!isStudent.value) return '登录后学习'
  if (locked.value) return '积分兑换'
  return resumeChapter.value ? '继续学习' : '开始学习'
})

async function begin(chapterId?: number) {
  const item = course.value
  if (!item || redeeming.value) return
  if (!isStudent.value) {
    const role = auth.isLoggedIn ? auth.userInfo?.role : undefined
    if (role === 'admin' || role === 'tutor') {
      try {
        await confirm.confirm('继续将切换此浏览器的共享登录身份。', '使用学员账号登录')
      } catch {
        return
      }
    }
    const redirect = router.resolve(href('PublicCourseDetail', { courseId: item.course_id })).path
    await router.push({ ...href('Login'), query: { redirect, course_auth: role ? '1' : undefined } })
    return
  }
  if (locked.value) {
    try {
      await confirm.confirm(`使用 ${item.points_price} 积分兑换这门课程？`, '积分兑换')
    } catch {
      return
    }
    redeeming.value = true
    try {
      await pointsApi.redeemCourse(item.course_id)
      if (courseId.value === item.course_id) await run()
    } catch {
      // 请求层统一报告兑换失败。
    } finally {
      redeeming.value = false
    }
    return
  }
  const nextChapter = chapterId ?? resumeChapter.value?.chapter_id ?? item.chapters[0]?.chapter_id
  if (nextChapter) {
    await router.push(href('ChapterView', { courseId: item.course_id, chapterId: nextChapter }))
  }
}
</script>
