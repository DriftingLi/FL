<template>
  <div class="tutor-dashboard">
    <!-- Welcome Banner -->
    <div class="welcome-banner">
      <div class="banner-content">
        <h1 class="banner-title">欢迎回来，{{ userName }}！</h1>
        <p class="banner-subtitle">管理你的课程与题库</p>
      </div>
    </div>

    <!-- 快捷卡片 -->
    <div class="quick-cards">
      <QuickCard
        title="我的课程"
        :items="myCourses"
        :max-items="100"
        more-link="/training/tutor/courses"
        empty-text="暂无课程"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useAuthStore } from '@/stores/auth'
import QuickCard from '@/components/dashboard/QuickCard.vue'
import type { QuickCardItem } from '@/components/dashboard/QuickCard.vue'
import { tutorApi } from '@/api/tutor'
import { displayNameOf } from '@/types/user'

const authStore = useAuthStore()

const userName = computed(() => displayNameOf(authStore.userInfo) || '导师')

const myCourses = ref<QuickCardItem[]>([])

async function loadData() {
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
    console.error('加载导师数据失败:', error)
  }
}

onMounted(async () => {
  await loadData()
})
</script>

<style scoped>
.tutor-dashboard {
  display: flex;
  flex-direction: column;
  gap: var(--space-6);
}

.welcome-banner {
  background: #ECFDF5;
  border: 1px solid #A7F3D0;
  border-radius: var(--radius-xl);
  padding: var(--space-6) var(--space-8);
  display: flex;
  align-items: center;
  justify-content: space-between;
  color: var(--color-text-primary);
}

.banner-title {
  font-family: var(--font-display);
  font-size: var(--text-2xl);
  font-weight: var(--font-bold);
  margin-bottom: var(--space-1);
  color: var(--color-text-primary);
}

.banner-subtitle {
  font-size: var(--text-base);
  color: var(--color-text-secondary);
}

.banner-action {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-3) var(--space-5);
  background: #059669;
  border: 1px solid #047857;
  border-radius: var(--radius-lg);
  color: white;
  font-size: var(--text-sm);
  font-weight: var(--font-medium);
  text-decoration: none;
  transition: background var(--duration-fast);
  white-space: nowrap;
}

.banner-action:hover {
  background: #047857;
}

.quick-cards {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: var(--space-4);
}

@media screen and (max-width: 1024px) {
  .quick-cards {
    grid-template-columns: repeat(2, 1fr);
  }

  .chart-area {
    height: 220px;
  }
}

@media screen and (max-width: 768px) {
  .welcome-banner {
    flex-direction: column;
    align-items: flex-start;
    gap: var(--space-4);
    padding: var(--space-5) var(--space-6);
  }

  .quick-cards {
    grid-template-columns: 1fr;
  }

  .banner-title {
    font-size: var(--text-xl);
  }
}
</style>
