<template>
  <div class="flex flex-col gap-6">
    <UiAsyncSection
      :error="pageError"
      :loading="pageLoading"
      :retrying="retrying"
      error-title="页面加载失败"
      error-description="网络或服务端异常，可重试"
      @retry="handleRetry"
    >
      <!-- 首屏骨架（横幅 + 卡片组） -->
      <template #skeleton>
        <div class="h-32 rounded-card border border-line bg-panel" />
        <UiSkeleton variant="card" :count="2" />
      </template>
      <!-- Welcome Banner -->
      <section
        class="rounded-card border border-ui-100 bg-gradient-to-br from-ui-50 to-panel p-6 sm:p-8"
      >
        <div class="flex flex-wrap items-center justify-between gap-4">
          <div class="min-w-0">
            <h1 class="font-heading text-2xl font-bold text-ink">
              欢迎回来，{{ userName }}！
            </h1>
            <p class="mt-1 text-sm text-ink-2">管理你的课程与题库</p>
          </div>

          <router-link
            :to="href('TutorCourses')"
            class="inline-flex items-center gap-1 rounded-ctl bg-ui-500 px-4 py-2 text-sm font-medium text-white transition-colors duration-150 hover:bg-ui-600"
          >
            管理课程
            <el-icon><ArrowRight /></el-icon>
          </router-link>
        </div>
      </section>

      <!-- 快捷卡片 -->
      <div class="grid gap-4 sm:grid-cols-2">
        <QuickCard
          title="我的课程"
          :items="myCourses"
          :max-items="100"
          :more-link="href('TutorCourses')"
          empty-text="暂无课程"
        />
      </div>
    </UiAsyncSection>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ArrowRight } from '@element-plus/icons-vue'
import { href } from '@/config/pages'
import { useAuthStore } from '@/stores/auth'
import QuickCard from '@/components/dashboard/QuickCard.vue'
import type { QuickCardItem } from '@/components/dashboard/QuickCard.vue'
import UiAsyncSection from '@/components/ui/UiAsyncSection.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'
import { tutorApi } from '@/api/tutor'
import { useAsyncPage } from '@/composables/useAsyncPage'
import { displayNameOf } from '@/types/user'

const authStore = useAuthStore()

const userName = computed(() => displayNameOf(authStore.userInfo) || '讲师')

const myCourses = ref<QuickCardItem[]>([])

// 三态收编 useAsyncPage（#401）：拦截器已 toast，pageError 降级 boolean；retry 防重入由 composable 提供
const {
  loading: pageLoading,
  loadError: pageError,
  retrying,
  retry: handleRetry,
  run: loadData
} = useAsyncPage(async () => {
  const courseRes = await tutorApi.getCourses({ page: 1, page_size: 100 })
  if (courseRes) {
    const courses = Array.isArray(courseRes) ? courseRes : (courseRes.courses || [])
    myCourses.value = courses.map((c) => ({
      title: c.name || '未命名课程',
      subtitle: `${c.student_count ?? 0} 名学员`,
      to: c.course_id ? href('TutorChapterManage', { id: String(c.course_id) }) : undefined
    }))
  }
})

onMounted(loadData)
</script>
