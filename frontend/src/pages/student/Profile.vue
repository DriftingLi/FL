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

      <el-tab-pane label="实操记录" name="practice">
        <div v-loading="practiceLoading" class="practice-container">
          <el-card class="filter-card">
            <el-row :gutter="16" align="middle">
              <el-col :span="6">
                <el-select v-model="practiceTypeFilter" placeholder="实操类型" clearable style="width: 100%;" @change="loadPracticeRecords">
                  <el-option label="日常检查" value="inspection" />
                  <el-option label="故障诊断" value="diagnosis" />
                  <el-option label="部件拆装" value="assembly" />
                </el-select>
              </el-col>
              <el-col :span="4">
                <el-button @click="loadPracticeRecords">刷新</el-button>
              </el-col>
            </el-row>
          </el-card>

          <el-card class="table-card">
            <el-table :data="practiceRecords" stripe style="width: 100%">
              <el-table-column prop="created_at" label="时间" width="170">
                <template #default="{ row }">
                  {{ formatDate(row.created_at) }}
                </template>
              </el-table-column>
              <el-table-column prop="practice_type" label="类型" width="120">
                <template #default="{ row }">
                  <el-tag :type="getTypeTagType(row.practice_type)" size="small">
                    {{ getTypeLabel(row.practice_type) }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="difficulty" label="难度" width="80">
                <template #default="{ row }">
                  {{ getDifficultyLabel(row.difficulty) }}
                </template>
              </el-table-column>
              <el-table-column prop="score" label="得分" width="80" align="center">
                <template #default="{ row }">
                  <span :style="{ color: row.score >= 80 ? '#67c23a' : row.score >= 60 ? '#e6a23c' : '#f56c6c', fontWeight: 'bold' }">
                    {{ row.score }}
                  </span>
                </template>
              </el-table-column>
              <el-table-column prop="duration" label="用时" width="100" align="center">
                <template #default="{ row }">
                  {{ row.duration }}秒
                </template>
              </el-table-column>
              <el-table-column prop="status" label="状态" width="80" align="center">
                <template #default="{ row }">
                  <el-tag :type="row.status === 'completed' ? 'success' : 'warning'" size="small">
                    {{ row.status === 'completed' ? '完成' : '未完成' }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column label="操作" width="120" align="center">
                <template #default="{ row }">
                  <el-button size="small" text type="primary" @click="viewPracticeDetail(row)">详情</el-button>
                </template>
              </el-table-column>
            </el-table>

            <div class="pagination-wrapper" v-if="practicePagination.total > 0">
              <el-pagination
                v-model:current-page="practicePage"
                :page-size="practicePageSize"
                :total="practicePagination.total"
                layout="total, prev, pager, next"
                @current-change="handlePracticePageChange"
              />
            </div>

            <el-empty v-if="practiceRecords.length === 0 && !practiceLoading" description="暂无实操记录，快去虚拟实操模块练习吧！">
              <el-button type="primary" @click="$router.push('/training/practice')">前往实操</el-button>
            </el-empty>
          </el-card>
        </div>

        <el-dialog v-model="detailDialogVisible" title="实操记录详情" width="600px">
          <div v-if="practiceDetail" class="detail-content">
            <el-descriptions :column="2" border>
              <el-descriptions-item label="实操类型">{{ getTypeLabel(practiceDetail.practice_type) }}</el-descriptions-item>
              <el-descriptions-item label="难度">{{ getDifficultyLabel(practiceDetail.difficulty) }}</el-descriptions-item>
              <el-descriptions-item label="得分">
                <span :style="{ color: practiceDetail.score >= 80 ? '#67c23a' : practiceDetail.score >= 60 ? '#e6a23c' : '#f56c6c', fontWeight: 'bold' }">
                  {{ practiceDetail.score }}分
                </span>
              </el-descriptions-item>
              <el-descriptions-item label="用时">{{ practiceDetail.duration }}秒</el-descriptions-item>
              <el-descriptions-item label="时间">{{ formatDate(practiceDetail.created_at) }}</el-descriptions-item>
              <el-descriptions-item label="状态">
                <el-tag :type="practiceDetail.status === 'completed' ? 'success' : 'warning'" size="small">
                  {{ practiceDetail.status === 'completed' ? '完成' : '未完成' }}
                </el-tag>
              </el-descriptions-item>
            </el-descriptions>

            <div v-if="practiceDetail.operations && practiceDetail.operations.length > 0" class="operations-section">
              <h4>操作记录 ({{ practiceDetail.operations.length }}步)</h4>
              <div class="operations-list">
                <div v-for="(op, index) in practiceDetail.operations" :key="index" class="op-item">
                  <span class="op-index">{{ index + 1 }}</span>
                  <span class="op-part">{{ op.partName || op.partId }}</span>
                  <el-tag size="small" :type="getActionTagType(op.action)">{{ getActionLabel(op.action) }}</el-tag>
                </div>
              </div>
            </div>
          </div>
        </el-dialog>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useUserStore } from '@/stores/user'
import { practiceApi } from '@/api/practice'

const router = useRouter()
const userStore = useUserStore()

const activeTab = ref('info')
const profileLoading = ref(false)
const recordsLoading = ref(false)
const dateRange = ref(null)
const currentPage = ref(1)
const pageSize = ref(10)

const practiceLoading = ref(false)
const practiceRecords = ref([])
const practicePagination = ref({ total: 0 })
const practicePage = ref(1)
const practicePageSize = ref(10)
const practiceTypeFilter = ref('')
const detailDialogVisible = ref(false)
const practiceDetail = ref(null)

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

function getTypeLabel(type) {
  const map = { inspection: '日常检查', diagnosis: '故障诊断', assembly: '部件拆装' }
  return map[type] || type
}

function getTypeTagType(type) {
  const map = { inspection: 'success', diagnosis: 'warning', assembly: '' }
  return map[type] || 'info'
}

function getDifficultyLabel(diff) {
  const map = { beginner: '初级', normal: '中级', expert: '高级' }
  return map[diff] || diff
}

function getActionLabel(action) {
  const map = { click: '点击', detach: '拆卸', attach: '装回', undo: '撤销' }
  return map[action] || action
}

function getActionTagType(action) {
  const map = { click: 'info', detach: 'warning', attach: 'success', undo: '' }
  return map[action] || 'info'
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

async function loadPracticeRecords() {
  practiceLoading.value = true
  try {
    const params: Record<string, any> = {
      page: practicePage.value,
      page_size: practicePageSize.value
    }
    if (practiceTypeFilter.value) {
      params.practice_type = practiceTypeFilter.value
    }
    const res = await practiceApi.getRecords(params)
    if (res.data) {
      practiceRecords.value = res.data.records || []
      practicePagination.value = { total: res.data.total || 0 }
    }
  } catch (e) {
    console.error('加载实操记录失败:', e)
  } finally {
    practiceLoading.value = false
  }
}

async function viewPracticeDetail(row) {
  try {
    const res = await practiceApi.getRecordDetail(row.record_id)
    if (res.data) {
      practiceDetail.value = res.data
      detailDialogVisible.value = true
    }
  } catch (e) {
    console.error('加载实操详情失败:', e)
  }
}

function handleTabChange(tab) {
  if (tab === 'info') {
    loadProfile()
  } else if (tab === 'records') {
    currentPage.value = 1
    loadRecords()
  } else if (tab === 'practice') {
    practicePage.value = 1
    loadPracticeRecords()
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

function handlePracticePageChange(page) {
  practicePage.value = page
  loadPracticeRecords()
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

.detail-content {
  max-height: 60vh;
  overflow-y: auto;
}

.operations-section {
  margin-top: 20px;
}

.operations-section h4 {
  font-size: 15px;
  color: #303133;
  margin-bottom: 12px;
}

.operations-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  max-height: 300px;
  overflow-y: auto;
}

.op-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 10px;
  background: #f5f7fa;
  border-radius: 4px;
  font-size: 13px;
}

.op-index {
  width: 24px;
  height: 24px;
  border-radius: 50%;
  background: #409eff;
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
  flex-shrink: 0;
}

.op-part {
  flex: 1;
  color: #303133;
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
