<template>
  <div class="profile-page">
    <el-tabs v-model="activeTab" @tab-change="handleTabChange">
      <el-tab-pane label="我的信息" name="info">
        <div v-loading="profileLoading" class="info-container">
          <el-row :gutter="20">
            <el-col :xs="24" :sm="8">
              <el-card class="avatar-card">
                <div class="avatar-section">
                  <div class="avatar-circle">
                    {{ avatarLetter }}
                  </div>
                  <h3 class="username">{{ profile.username || '-' }}</h3>
                  <p class="name">{{ profile.name || '-' }}</p>
                  <el-tag v-if="profile.level" :type="levelTagType" class="level-tag">
                    {{ levelLabel }}
                  </el-tag>
                </div>
              </el-card>
            </el-col>
            <el-col :xs="24" :sm="16">
              <el-card class="stats-card">
                <template #header>
                  <span class="card-title">学习概览</span>
                </template>
                <el-row :gutter="20">
                  <el-col :span="6">
                    <div class="stat-item">
                      <div class="stat-value">{{ formatDuration(studyStats.total_study_duration) }}</div>
                      <div class="stat-label">总学习时长</div>
                    </div>
                  </el-col>
                  <el-col :span="6">
                    <div class="stat-item">
                      <div class="stat-value">{{ studyStats.completed_courses || 0 }}</div>
                      <div class="stat-label">已完成课程</div>
                    </div>
                  </el-col>
                  <el-col :span="6">
                    <div class="stat-item">
                      <div class="stat-value">{{ studyStats.learning_courses || 0 }}</div>
                      <div class="stat-label">学习中课程</div>
                    </div>
                  </el-col>
                  <el-col :span="6">
                    <div class="stat-item">
                      <div class="stat-value">{{ studyStats.avg_score || 0 }}</div>
                      <div class="stat-label">平均考核分</div>
                    </div>
                  </el-col>
                </el-row>
              </el-card>

              <el-card class="detail-card">
                <template #header>
                  <span class="card-title">基本信息</span>
                </template>
                <el-descriptions :column="2" border>
                  <el-descriptions-item label="用户名">{{ profile.username || '-' }}</el-descriptions-item>
                  <el-descriptions-item label="姓名">{{ profile.name || '-' }}</el-descriptions-item>
                  <el-descriptions-item label="当前等级">
                    <el-tag v-if="profile.level" :type="levelTagType" size="small">{{ levelLabel }}</el-tag>
                    <span v-else>初级学徒</span>
                  </el-descriptions-item>
                  <el-descriptions-item label="注册时间">{{ formatDate(profile.created_at) }}</el-descriptions-item>
                  <el-descriptions-item label="最近学习">{{ studyStats.latest_study_time ? formatDate(studyStats.latest_study_time) : '暂无' }}</el-descriptions-item>
                  <el-descriptions-item label="考核次数">{{ studyStats.exam_count || 0 }}</el-descriptions-item>
                  <el-descriptions-item label="账号状态">
                    <el-tag :type="profile.status === 1 ? 'success' : 'danger'" size="small">
                      {{ profile.status === 1 ? '正常' : '禁用' }}
                    </el-tag>
                  </el-descriptions-item>
                </el-descriptions>
              </el-card>
            </el-col>
          </el-row>

          <el-card class="progress-card" v-if="courseProgress.length > 0">
            <template #header>
              <span class="card-title">课程学习进度</span>
            </template>
            <div class="progress-list">
              <div v-for="item in courseProgress" :key="item.course_id" class="progress-item">
                <div class="progress-header">
                  <span class="course-name">{{ item.course_name }}</span>
                  <el-tag size="small" type="info">{{ getCategoryLabel(item.category) }}</el-tag>
                </div>
                <el-progress
                  :percentage="item.progress"
                  :color="getProgressColor(item.progress)"
                  :stroke-width="18"
                  :text-inside="true"
                  style="margin-top: 8px;"
                />
                <div class="progress-meta">
                  <span>学习时长: {{ formatDuration(item.study_duration) }}</span>
                  <span>共 {{ item.total_chapters }} 个章节</span>
                </div>
              </div>
            </div>
          </el-card>

          <el-card v-else class="empty-progress-card">
            <el-empty description="暂无学习记录，快去课程中心开始学习吧！">
              <el-button type="primary" @click="$router.push('/training')">前往课程中心</el-button>
            </el-empty>
          </el-card>
        </div>
      </el-tab-pane>

      <el-tab-pane label="学习记录" name="records">
        <div v-loading="recordsLoading" class="records-container">
          <el-card class="filter-card">
            <el-row :gutter="16" align="middle">
              <el-col :span="8">
                <el-date-picker
                  v-model="dateRange"
                  type="daterange"
                  range-separator="至"
                  start-placeholder="开始日期"
                  end-placeholder="结束日期"
                  value-format="YYYY-MM-DD"
                  style="width: 100%;"
                  @change="handleDateChange"
                />
              </el-col>
              <el-col :span="4">
                <el-button @click="resetDateFilter" :disabled="!dateRange">重置筛选</el-button>
              </el-col>
            </el-row>
          </el-card>

          <el-card class="table-card">
            <el-table :data="studyRecords" stripe style="width: 100%">
              <el-table-column prop="study_date" label="学习日期" width="180">
                <template #default="{ row }">
                  {{ formatDate(row.study_date) }}
                </template>
              </el-table-column>
              <el-table-column prop="course_name" label="课程名称" min-width="160" />
              <el-table-column prop="chapter_title" label="章节" min-width="140">
                <template #default="{ row }">
                  {{ row.chapter_title || '-' }}
                </template>
              </el-table-column>
              <el-table-column prop="study_duration" label="学习时长" width="120">
                <template #default="{ row }">
                  {{ formatDuration(row.study_duration) }}
                </template>
              </el-table-column>
              <el-table-column prop="progress" label="学习进度" width="180">
                <template #default="{ row }">
                  <el-progress
                    :percentage="row.progress"
                    :color="getProgressColor(row.progress)"
                    :stroke-width="14"
                    :text-inside="true"
                  />
                </template>
              </el-table-column>
            </el-table>

            <div class="pagination-wrapper" v-if="recordsPagination.total > 0">
              <el-pagination
                v-model:current-page="currentPage"
                :page-size="pageSize"
                :total="recordsPagination.total"
                layout="total, prev, pager, next"
                @current-change="handlePageChange"
              />
            </div>

            <el-empty v-if="studyRecords.length === 0 && !recordsLoading" description="暂无学习记录" />
          </el-card>
        </div>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useUserStore } from '@/stores/user'

const userStore = useUserStore()

const activeTab = ref('info')
const profileLoading = ref(false)
const recordsLoading = ref(false)
const dateRange = ref(null)
const currentPage = ref(1)
const pageSize = ref(10)

const profile = computed(() => userStore.profile)
const studyStats = computed(() => userStore.studyStats)
const courseProgress = computed(() => userStore.courseProgress)
const studyRecords = computed(() => userStore.studyRecords)
const recordsPagination = computed(() => userStore.recordsPagination)

const avatarLetter = computed(() => {
  const name = profile.value.name || profile.value.username || ''
  return name.charAt(0).toUpperCase()
})

const levelLabel = computed(() => {
  const map = { beginner: '初级学徒', intermediate: '中级学徒', advanced: '高级学徒', expert: '顶级学徒' }
  return map[profile.value.level] || ''
})

const levelTagType = computed(() => {
  const map = { beginner: 'success', intermediate: 'warning', advanced: 'danger' }
  return map[profile.value.level] || 'info'
})

const categoryMap = {
  'CATEGORY_01': '基础理论',
  'CATEGORY_02': '安全规范',
  'CATEGORY_03': '实操技能',
  'CATEGORY_04': '进阶提升'
}

function getCategoryLabel(category) {
  return categoryMap[category] || category || '未分类'
}

function formatDuration(minutes) {
  if (!minutes || minutes <= 0) return '0分钟'
  const hours = Math.floor(minutes / 60)
  const mins = minutes % 60
  if (hours > 0) {
    return mins > 0 ? `${hours}小时${mins}分钟` : `${hours}小时`
  }
  return `${mins}分钟`
}

function formatDate(dateStr) {
  if (!dateStr) return '-'
  try {
    const d = new Date(dateStr)
    if (isNaN(d.getTime())) return dateStr
    return d.toLocaleString('zh-CN', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit'
    })
  } catch {
    return dateStr
  }
}

function getProgressColor(progress) {
  if (progress >= 100) return '#67c23a'
  if (progress >= 60) return '#409eff'
  if (progress >= 30) return '#e6a23c'
  return '#f56c6c'
}

async function loadProfile() {
  profileLoading.value = true
  try {
    await userStore.fetchProfile()
  } finally {
    profileLoading.value = false
  }
}

async function loadRecords() {
  recordsLoading.value = true
  const params: Record<string, any> = {
    page: currentPage.value,
    page_size: pageSize.value
  }
  if (dateRange.value && dateRange.value.length === 2) {
    params.start_date = dateRange.value[0]
    params.end_date = dateRange.value[1]
  }
  try {
    await userStore.fetchRecords(params)
  } finally {
    recordsLoading.value = false
  }
}

function handleTabChange(tab) {
  if (tab === 'info') {
    loadProfile()
  } else if (tab === 'records') {
    currentPage.value = 1
    loadRecords()
  }
}

function handleDateChange() {
  currentPage.value = 1
  loadRecords()
}

function resetDateFilter() {
  dateRange.value = null
  currentPage.value = 1
  loadRecords()
}

function handlePageChange(page) {
  currentPage.value = page
  loadRecords()
}

onMounted(() => {
  loadProfile()
})
</script>

<style scoped>
.profile-page {
  max-width: 1200px;
  margin: 0 auto;
}

.info-container,
.records-container,
.practice-container {
  min-height: 400px;
}

.avatar-card {
  text-align: center;
  margin-bottom: 20px;
}

.avatar-section {
  padding: 20px 0;
}

.avatar-circle {
  width: 80px;
  height: 80px;
  border-radius: 50%;
  background: linear-gradient(135deg, #409eff, #667eea);
  color: #fff;
  font-size: 36px;
  font-weight: bold;
  display: flex;
  align-items: center;
  justify-content: center;
  margin: 0 auto 16px;
}

.username {
  font-size: 18px;
  color: #303133;
  margin-bottom: 4px;
}

.name {
  color: #909399;
  font-size: 14px;
}

.level-tag {
  margin-top: 8px;
}

.stats-card {
  margin-bottom: 20px;
}

.card-title {
  font-size: 16px;
  font-weight: 600;
  color: #303133;
}

.stat-item {
  text-align: center;
  padding: 10px 0;
}

.stat-value {
  font-size: 24px;
  font-weight: bold;
  color: #409eff;
  margin-bottom: 6px;
}

.stat-label {
  font-size: 13px;
  color: #909399;
}

.detail-card {
  margin-bottom: 20px;
}

.progress-card,
.empty-progress-card {
  margin-top: 20px;
}

.progress-list {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.progress-item {
  padding: 12px 0;
  border-bottom: 1px solid #f0f0f0;
}

.progress-item:last-child {
  border-bottom: none;
}

.progress-header {
  display: flex;
  align-items: center;
  gap: 10px;
}

.course-name {
  font-size: 15px;
  font-weight: 500;
  color: #303133;
}

.progress-meta {
  display: flex;
  gap: 20px;
  margin-top: 6px;
  font-size: 13px;
  color: #909399;
}

.filter-card {
  margin-bottom: 16px;
}

.table-card {
  margin-bottom: 20px;
}

.pagination-wrapper {
  display: flex;
  justify-content: center;
  margin-top: 20px;
}

@media screen and (max-width: 768px) {
  .stat-value {
    font-size: 20px;
  }

  .avatar-card {
    margin-bottom: 12px;
  }

  .filter-card :deep(.el-col) {
    margin-bottom: 8px;
  }

  .filter-card :deep(.el-col:last-child) {
    margin-bottom: 0;
  }

  .progress-meta {
    flex-direction: column;
    gap: 4px;
  }

  .detail-card :deep(.el-descriptions) {
    --el-descriptions-item-bordered-label-background: #f5f7fa;
  }
}

@media screen and (max-width: 480px) {
  .username {
    font-size: 16px;
  }

  .stat-value {
    font-size: 18px;
  }

  .stat-label {
    font-size: 12px;
  }
}
</style>
