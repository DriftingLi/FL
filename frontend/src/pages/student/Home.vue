<template>
  <div class="home-page">
    <section class="hero-section">
      <div class="hero-content">
        <div class="hero-text">
          <p class="hero-greeting">{{ greeting }}，</p>
          <h1 class="hero-name">{{ authStore.userInfo.name || authStore.userInfo.username }}</h1>
          <p class="hero-desc">继续您的叉车维修学习之旅</p>
        </div>
        <div class="hero-visual">
          <div class="progress-ring">
            <svg viewBox="0 0 120 120" class="ring-svg">
              <circle cx="60" cy="60" r="52" fill="none" stroke="rgba(255,255,255,0.15)" stroke-width="8"/>
              <circle cx="60" cy="60" r="52" fill="none" stroke="white" stroke-width="8"
                stroke-linecap="round"
                :stroke-dasharray="circumference"
                :stroke-dashoffset="circumference * (1 - overallProgress / 100)"
                transform="rotate(-90 60 60)"
                class="ring-progress"
              />
            </svg>
            <div class="ring-label">
              <span class="ring-value">{{ Math.round(overallProgress) }}%</span>
              <span class="ring-text">学习进度</span>
            </div>
          </div>
        </div>
      </div>
      <div class="hero-decor"></div>
    </section>

    <section class="stats-section">
      <div class="stat-card" v-for="stat in stats" :key="stat.label">
        <div class="stat-icon-wrap" :style="{ background: stat.bgColor }">
          <el-icon :size="22" :style="{ color: stat.color }"><component :is="stat.icon" /></el-icon>
        </div>
        <div class="stat-info">
          <div class="stat-value">{{ stat.value }}</div>
          <div class="stat-label">{{ stat.label }}</div>
        </div>
      </div>
    </section>

    <section class="features-section">
      <div class="feature-card" v-for="feature in features" :key="feature.label" @click="$router.push(feature.path)">
        <div class="feature-header">
          <div class="feature-icon-wrap" :style="{ background: feature.gradient }">
            <el-icon :size="24" color="white"><component :is="feature.icon" /></el-icon>
          </div>
          <el-icon class="feature-arrow"><ArrowRight /></el-icon>
        </div>
        <h3 class="feature-title">{{ feature.label }}</h3>
        <p class="feature-desc">{{ feature.desc }}</p>
      </div>
    </section>

    <section class="bottom-section">
      <div class="recent-section">
        <div class="section-header">
          <h3>最近学习</h3>
          <router-link to="/courses" class="view-all">查看全部</router-link>
        </div>
        <div class="recent-list">
          <div class="recent-item" v-for="item in recentItems" :key="item.id">
            <div class="recent-icon">
              <el-icon :size="18"><Notebook /></el-icon>
            </div>
            <div class="recent-info">
              <div class="recent-name">{{ item.name }}</div>
              <div class="recent-chapter">{{ item.chapter }}</div>
            </div>
            <div class="recent-progress">
              <el-progress :percentage="item.progress" :stroke-width="6" :show-text="false" color="var(--color-primary-500)" />
              <span class="progress-text">{{ item.progress }}%</span>
            </div>
          </div>
          <div v-if="recentItems.length === 0" class="empty-hint">
            <p>暂无学习记录</p>
            <router-link to="/courses" class="start-link">开始学习 →</router-link>
          </div>
        </div>
      </div>

      <div class="exam-section">
        <div class="section-header">
          <h3>待完成考试</h3>
        </div>
        <div class="exam-list">
          <div class="exam-item" v-for="exam in upcomingExams" :key="exam.id">
            <div class="exam-badge">{{ exam.level }}</div>
            <div class="exam-info">
              <div class="exam-name">{{ exam.name }}</div>
              <div class="exam-time">{{ exam.time }}</div>
            </div>
          </div>
          <div v-if="upcomingExams.length === 0" class="empty-hint">
            <p>暂无待完成考试</p>
          </div>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useAuthStore } from '@/stores/auth'
import request from '@/api/request'
import { Notebook, MagicStick, SetUp, EditPen, Timer, ArrowRight } from '@element-plus/icons-vue'

const authStore = useAuthStore()

const circumference = 2 * Math.PI * 52

const currentHour = ref(new Date().getHours())

const greeting = computed(() => {
  const hour = currentHour.value
  if (hour >= 6 && hour < 12) return '早上好'
  if (hour >= 12 && hour < 14) return '中午好'
  if (hour >= 14 && hour < 18) return '下午好'
  return '晚上好'
})

let greetingTimer = null

const overallProgress = ref(0)

const stats = ref([
  { label: '已学课程', value: '0', icon: Notebook, color: '#2563EB', bgColor: '#EFF6FF' },
  { label: '学习时长', value: '0h', icon: Timer, color: '#10B981', bgColor: '#ECFDF5' },
  { label: '练习题数', value: '0', icon: EditPen, color: '#F59E0B', bgColor: '#FFFBEB' },
  { label: '实操次数', value: '0', icon: SetUp, color: '#8B5CF6', bgColor: '#F5F3FF' }
])

const features = [
  {
    label: '课程中心',
    desc: '浏览和学习各类叉车维修课程',
    icon: Notebook,
    path: '/courses',
    gradient: 'linear-gradient(135deg, #2563EB, #3B82F6)'
  },
  {
    label: 'AI 智能助手',
    desc: '智能生成维修知识和故障诊断',
    icon: MagicStick,
    path: '/ai-generate',
    gradient: 'linear-gradient(135deg, #8B5CF6, #A78BFA)'
  },
  {
    label: '虚拟实操',
    desc: '3D 场景下的叉车维修实操训练',
    icon: SetUp,
    path: '/practice',
    gradient: 'linear-gradient(135deg, #EA580C, #F97316)'
  }
]

const recentItems = ref([])
const upcomingExams = ref([])

const levelLabelMap = {
  beginner: '初级',
  intermediate: '中级',
  advanced: '高级',
  expert: '顶级'
}

function formatDuration(minutes) {
  if (!minutes || minutes <= 0) return '0h'
  const hours = Math.floor(minutes / 60)
  const mins = Math.round(minutes % 60)
  if (hours > 0) {
    return mins > 0 ? `${hours}h${mins}m` : `${hours}h`
  }
  return `${mins}m`
}

async function loadHomeData() {
  const silentGet = (url) => request.get(url, { headers: { 'X-Silent': 'true' } })

  const safeGet = async (url, retries = 2) => {
    for (let attempt = 0; attempt <= retries; attempt++) {
      try {
        const res = await silentGet(url)
        if (res && (res.code === 200 || res.code === 201)) {
          return res.data !== undefined && res.data !== null ? res.data : null
        }
        if (attempt < retries) {
          await new Promise(r => setTimeout(r, 300 * (attempt + 1)))
          continue
        }
        return null
      } catch (e) {
        if (attempt < retries) {
          await new Promise(r => setTimeout(r, 300 * (attempt + 1)))
          continue
        }
        return null
      }
    }
    return null
  }

  const loadProfile = async () => {
    const data = await safeGet('/student/profile')
    if (!data || typeof data !== 'object') return

    const studyStats = data.study_stats || {}
    const courseProgress = Array.isArray(data.course_progress) ? data.course_progress : []

    const completedCourses = Number(studyStats.completed_courses) || 0
    const learningCourses = Number(studyStats.learning_courses) || 0
    const totalStudyDuration = Number(studyStats.total_study_duration) || 0

    stats.value[0].value = String(completedCourses + learningCourses)
    stats.value[1].value = formatDuration(totalStudyDuration)

    const coursesData = await safeGet('/courses')
    let totalCoursesCount = completedCourses + learningCourses
    if (coursesData && typeof coursesData === 'object') {
      const apiTotal = Number(coursesData.total)
      if (apiTotal > 0) {
        totalCoursesCount = apiTotal
      }
    }

    if (totalCoursesCount > 0) {
      const completedProgress = courseProgress.reduce((sum, c) => sum + (Number(c.progress) || 0), 0)
      const unstartedCourses = Math.max(0, totalCoursesCount - courseProgress.length)
      overallProgress.value = (completedProgress + unstartedCourses * 0) / totalCoursesCount
    } else if (courseProgress.length > 0) {
      const avgProgress = courseProgress.reduce((sum, c) => sum + (Number(c.progress) || 0), 0) / courseProgress.length
      overallProgress.value = avgProgress
    }

    recentItems.value = [...courseProgress]
      .sort((a, b) => {
        const dateA = a.study_date ? new Date(a.study_date).getTime() : 0
        const dateB = b.study_date ? new Date(b.study_date).getTime() : 0
        return dateB - dateA
      })
      .slice(0, 5)
      .map(c => ({
        id: c.course_id,
        name: c.course_name || '未知课程',
        chapter: `${c.total_chapters || 0} 章节`,
        progress: Math.round(Number(c.progress) || 0)
      }))
  }

  const loadPracticeStats = async () => {
    const data = await safeGet('/practice/stats')
    if (data && typeof data === 'object') {
      stats.value[3].value = String(Number(data.total_count) || 0)
    }
  }

  const loadPracticeModeStats = async () => {
    const data = await safeGet('/practice-mode/stats')
    if (data && typeof data === 'object') {
      stats.value[2].value = String(Number(data.total) || 0)
    }
  }

  const loadExams = async () => {
    const data = await safeGet('/level-exam/available')
    if (data) {
      const exams = Array.isArray(data) ? data : []
      upcomingExams.value = exams
        .filter(e => !e.has_participated && (e.status === 'upcoming' || e.status === 'ongoing'))
        .map(e => ({
          id: e.id || e.session_id,
          level: levelLabelMap[e.level] || e.level || '等级考',
          name: (e.name && isNaN(e.name)) ? e.name : `${levelLabelMap[e.level] || ''}等级考试`,
          time: e.start_time ? new Date(e.start_time).toLocaleString('zh-CN') : '待定'
        }))
    }
  }

  await Promise.allSettled([
    loadProfile(),
    loadPracticeStats(),
    loadPracticeModeStats(),
    loadExams()
  ])
}

onMounted(() => {
  loadHomeData()
  greetingTimer = setInterval(() => {
    currentHour.value = new Date().getHours()
  }, 60000)
})

onUnmounted(() => {
  if (greetingTimer) {
    clearInterval(greetingTimer)
    greetingTimer = null
  }
})
</script>

<style scoped>
.home-page {
  display: flex;
  flex-direction: column;
  gap: var(--space-6);
}

.hero-section {
  background: var(--gradient-brand);
  border-radius: var(--radius-2xl);
  padding: var(--space-10) var(--space-8);
  position: relative;
  overflow: hidden;
}

.hero-content {
  display: flex;
  align-items: center;
  justify-content: space-between;
  position: relative;
  z-index: 2;
}

.hero-greeting {
  font-size: var(--text-lg);
  color: rgba(255, 255, 255, 0.8);
  margin-bottom: var(--space-1);
}

.hero-name {
  font-family: var(--font-display);
  font-size: var(--text-4xl);
  font-weight: var(--font-bold);
  color: white;
  margin-bottom: var(--space-2);
}

.hero-desc {
  font-size: var(--text-base);
  color: rgba(255, 255, 255, 0.7);
}

.hero-visual {
  flex-shrink: 0;
}

.progress-ring {
  position: relative;
  width: 120px;
  height: 120px;
}

.ring-svg {
  width: 100%;
  height: 100%;
}

.ring-progress {
  transition: stroke-dashoffset 1s var(--ease-out);
}

.ring-label {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
}

.ring-value {
  font-family: var(--font-display);
  font-size: var(--text-2xl);
  font-weight: var(--font-bold);
  color: white;
  line-height: 1;
}

.ring-text {
  font-size: var(--text-xs);
  color: rgba(255, 255, 255, 0.7);
  margin-top: 2px;
}

.hero-decor {
  position: absolute;
  width: 300px;
  height: 300px;
  border-radius: var(--radius-full);
  border: 1px solid rgba(255, 255, 255, 0.06);
  right: -60px;
  top: -60px;
  pointer-events: none;
}

.stats-section {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: var(--space-4);
}

.stat-card {
  background: var(--color-bg-card);
  border-radius: var(--radius-lg);
  padding: var(--space-5);
  display: flex;
  align-items: center;
  gap: var(--space-4);
  box-shadow: var(--shadow-xs);
  border: 1px solid var(--color-border-light);
  transition: all var(--duration-normal) var(--ease-default);
}

.stat-card:hover {
  box-shadow: var(--shadow-md);
  transform: translateY(-2px);
}

.stat-icon-wrap {
  width: 44px;
  height: 44px;
  border-radius: var(--radius-lg);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.stat-value {
  font-family: var(--font-display);
  font-size: var(--text-2xl);
  font-weight: var(--font-bold);
  color: var(--color-text-primary);
  line-height: 1.2;
}

.stat-label {
  font-size: var(--text-sm);
  color: var(--color-text-tertiary);
  margin-top: 2px;
}

.features-section {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: var(--space-4);
}

.feature-card {
  background: var(--color-bg-card);
  border-radius: var(--radius-xl);
  padding: var(--space-6);
  cursor: pointer;
  box-shadow: var(--shadow-xs);
  border: 1px solid var(--color-border-light);
  transition: all var(--duration-normal) var(--ease-default);
}

.feature-card:hover {
  box-shadow: var(--shadow-lg);
  transform: translateY(-4px);
}

.feature-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: var(--space-4);
}

.feature-icon-wrap {
  width: 48px;
  height: 48px;
  border-radius: var(--radius-lg);
  display: flex;
  align-items: center;
  justify-content: center;
}

.feature-arrow {
  color: var(--color-text-disabled);
  transition: all var(--duration-fast) var(--ease-default);
}

.feature-card:hover .feature-arrow {
  color: var(--color-primary-500);
  transform: translateX(4px);
}

.feature-title {
  font-size: var(--text-lg);
  font-weight: var(--font-semibold);
  color: var(--color-text-primary);
  margin-bottom: var(--space-2);
}

.feature-desc {
  font-size: var(--text-sm);
  color: var(--color-text-tertiary);
  line-height: var(--leading-relaxed);
}

.bottom-section {
  display: grid;
  grid-template-columns: 1.5fr 1fr;
  gap: var(--space-4);
}

.recent-section,
.exam-section {
  background: var(--color-bg-card);
  border-radius: var(--radius-xl);
  padding: var(--space-6);
  box-shadow: var(--shadow-xs);
  border: 1px solid var(--color-border-light);
}

.section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: var(--space-4);
}

.section-header h3 {
  font-size: var(--text-lg);
  font-weight: var(--font-semibold);
  color: var(--color-text-primary);
}

.view-all {
  font-size: var(--text-sm);
  color: var(--color-primary-500);
  font-weight: var(--font-medium);
  text-decoration: none;
  transition: color var(--duration-fast);
}

.view-all:hover {
  color: var(--color-primary-600);
}

.recent-list {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}

.recent-item {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-3);
  border-radius: var(--radius-lg);
  transition: background var(--duration-fast);
}

.recent-item:hover {
  background: var(--color-bg-page);
}

.recent-icon {
  width: 36px;
  height: 36px;
  border-radius: var(--radius-md);
  background: var(--color-primary-50);
  color: var(--color-primary-500);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.recent-info {
  flex: 1;
  min-width: 0;
}

.recent-name {
  font-size: var(--text-sm);
  font-weight: var(--font-medium);
  color: var(--color-text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.recent-chapter {
  font-size: var(--text-xs);
  color: var(--color-text-tertiary);
}

.recent-progress {
  width: 100px;
  display: flex;
  align-items: center;
  gap: var(--space-2);
  flex-shrink: 0;
}

.recent-progress :deep(.el-progress) {
  flex: 1;
}

.progress-text {
  font-size: var(--text-xs);
  font-weight: var(--font-medium);
  color: var(--color-text-tertiary);
  min-width: 30px;
  text-align: right;
}

.exam-list {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}

.exam-item {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-3);
  border-radius: var(--radius-lg);
  transition: background var(--duration-fast);
}

.exam-item:hover {
  background: var(--color-bg-page);
}

.exam-badge {
  padding: var(--space-1) var(--space-3);
  border-radius: var(--radius-full);
  background: var(--color-accent-50);
  color: var(--color-accent-600);
  font-size: var(--text-xs);
  font-weight: var(--font-semibold);
  flex-shrink: 0;
}

.exam-name {
  font-size: var(--text-sm);
  font-weight: var(--font-medium);
  color: var(--color-text-primary);
}

.exam-time {
  font-size: var(--text-xs);
  color: var(--color-text-tertiary);
}

.empty-hint {
  text-align: center;
  padding: var(--space-6) 0;
  color: var(--color-text-tertiary);
  font-size: var(--text-sm);
}

.start-link {
  display: inline-block;
  margin-top: var(--space-2);
  color: var(--color-primary-500);
  font-weight: var(--font-medium);
  text-decoration: none;
}

@media screen and (max-width: 768px) {
  .hero-section {
    padding: var(--space-6) var(--space-5);
  }

  .hero-name {
    font-size: var(--text-2xl);
  }

  .hero-visual {
    display: none;
  }

  .stats-section {
    grid-template-columns: repeat(2, 1fr);
  }

  .features-section {
    grid-template-columns: 1fr;
  }

  .bottom-section {
    grid-template-columns: 1fr;
  }
}

@media screen and (max-width: 480px) {
  .hero-section {
    padding: var(--space-5) var(--space-4);
    border-radius: var(--radius-xl);
  }

  .hero-name {
    font-size: var(--text-xl);
  }

  .stats-section {
    gap: var(--space-3);
  }

  .stat-card {
    padding: var(--space-4);
  }

  .stat-value {
    font-size: var(--text-xl);
  }
}
</style>
