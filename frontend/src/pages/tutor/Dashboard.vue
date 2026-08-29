<template>
  <div class="flex flex-col gap-6">
    <!-- 首屏骨架 -->
    <template v-if="pageLoading">
      <div class="h-32 rounded-card border border-line bg-panel" />
      <UiSkeleton variant="card" :count="2" />
    </template>

    <!-- 整页错误态：拦截器已 toast，这里只给可操作的重试入口 -->
    <UiErrorState
      v-else-if="pageError"
      title="页面加载失败"
      description="网络或服务端异常，可重试"
      :retrying="retrying"
      @retry="handleRetry"
    />

    <template v-else>
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
            to="/training/tutor/courses"
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
          more-link="/training/tutor/courses"
          empty-text="暂无课程"
        />
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ArrowRight } from '@element-plus/icons-vue'
import { useAuthStore } from '@/stores/auth'
import QuickCard from '@/components/dashboard/QuickCard.vue'
import type { QuickCardItem } from '@/components/dashboard/QuickCard.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'
import UiErrorState from '@/components/ui/UiErrorState.vue'
import { tutorApi } from '@/api/tutor'
import { displayNameOf } from '@/types/user'

const authStore = useAuthStore()

const userName = computed(() => displayNameOf(authStore.userInfo) || '导师')

const myCourses = ref<QuickCardItem[]>([])
const pageLoading = ref(true)
const pageError = ref('')
const retrying = ref(false)

async function loadData() {
  pageError.value = ''
  try {
    const courseRes = await tutorApi.getCourses({ page: 1, page_size: 100 })
    if (courseRes) {
      const courses = Array.isArray(courseRes) ? courseRes : (courseRes.courses || [])
      myCourses.value = courses.map((c) => ({
        title: c.name || '未命名课程',
        subtitle: `${c.student_count ?? 0} 名学员`,
        path: c.course_id ? `/training/tutor/course/${c.course_id}/chapters` : ''
      }))
    }
  } catch (error) {
    // 拦截器已统一 toast，这里只保留一个可操作的重试入口，避免静默失败后页面空白
    pageError.value = error instanceof Error ? error.message : '加载导师数据失败'
  } finally {
    pageLoading.value = false
  }
}

async function handleRetry() {
  retrying.value = true
  pageLoading.value = true
  try {
    await loadData()
  } finally {
    retrying.value = false
  }
}

onMounted(loadData)
</script>
