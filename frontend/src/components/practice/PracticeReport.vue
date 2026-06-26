<template>
  <div class="practice-report">
    <div class="report-header">
      <h3>实操评估报告</h3>
      <el-button size="small" text @click="$emit('close')">✕</el-button>
    </div>

    <div class="report-body">
      <div class="score-summary">
        <div class="score-circle" :style="{ '--score-color': scoreColor }">
          <span class="score-value">{{ stats.avg_score || 0 }}</span>
          <span class="score-label">平均分</span>
        </div>
        <div class="summary-stats">
          <div class="summary-item">
            <span class="summary-value">{{ stats.total_count || 0 }}</span>
            <span class="summary-label">练习次数</span>
          </div>
          <div class="summary-item">
            <span class="summary-value">{{ formatDuration(stats.total_duration || 0) }}</span>
            <span class="summary-label">总用时</span>
          </div>
        </div>
      </div>

      <div class="radar-section">
        <h4>技能评估</h4>
        <div ref="radarChartRef" class="radar-chart"></div>
      </div>

      <div v-if="stats.type_stats && Object.keys(stats.type_stats).length > 0" class="type-section">
        <h4>各类型表现</h4>
        <div class="type-bars">
          <div v-for="(info, type) in stats.type_stats" :key="type" class="type-bar-item">
            <div class="type-bar-header">
              <span class="type-name">{{ getTypeLabel(type) }}</span>
              <span class="type-score">{{ info.avg_score }}分</span>
            </div>
            <el-progress
              :percentage="info.avg_score"
              :color="getTypeColor(type)"
              :stroke-width="12"
              :text-inside="true"
            />
            <span class="type-count">{{ info.count }}次 | {{ formatDuration(info.total_duration) }}</span>
          </div>
        </div>
      </div>

      <div v-if="stats.difficulty_stats && Object.keys(stats.difficulty_stats).length > 0" class="difficulty-section">
        <h4>难度分布</h4>
        <div class="difficulty-bars">
          <div v-for="(info, diff) in stats.difficulty_stats" :key="diff" class="difficulty-item">
            <span class="diff-label">{{ getDifficultyLabel(diff) }}</span>
            <el-progress
              :percentage="info.avg_score"
              :color="getDifficultyColor(diff)"
              :stroke-width="10"
              style="flex: 1;"
            />
            <span class="diff-score">{{ info.avg_score }}分 ({{ info.count }}次)</span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick, watch } from 'vue'
import * as echarts from 'echarts'
import { practiceApi } from '@/api/practice'

const emit = defineEmits(['close'])

const radarChartRef = ref(null)
let radarChart = null
const stats = ref({})

const scoreColor = computed(() => {
  const s = stats.value.avg_score || 0
  if (s >= 80) return '#67c23a'
  if (s >= 60) return '#e6a23c'
  return '#f56c6c'
})

function getTypeLabel(type) {
  const map = { inspection: '日常检查', diagnosis: '故障诊断', assembly: '部件拆装' }
  return map[type] || type
}

function getTypeColor(type) {
  const map = { inspection: '#67c23a', diagnosis: '#e6a23c', assembly: '#409eff' }
  return map[type] || '#909399'
}

function getDifficultyLabel(diff) {
  const map = { beginner: '初级', normal: '中级', expert: '高级' }
  return map[diff] || diff
}

function getDifficultyColor(diff) {
  const map = { beginner: '#67c23a', normal: '#409eff', expert: '#f56c6c' }
  return map[diff] || '#909399'
}

function formatDuration(seconds) {
  if (!seconds || seconds <= 0) return '0秒'
  const mins = Math.floor(seconds / 60)
  const secs = seconds % 60
  if (mins > 0) return `${mins}分${secs}秒`
  return `${secs}秒`
}

function initRadarChart() {
  if (!radarChartRef.value) return

  if (radarChart) radarChart.dispose()
  radarChart = echarts.init(radarChartRef.value)

  const skillScores = stats.value.skill_scores || {}
  const indicators = []
  const values = []

  const defaultSkills = [
    { key: 'inspection', label: '日常检查' },
    { key: 'diagnosis', label: '故障诊断' },
    { key: 'assembly', label: '部件拆装' },
    { key: 'speed', label: '操作速度' },
    { key: 'accuracy', label: '操作准确' }
  ]

  defaultSkills.forEach(skill => {
    const data = skillScores[skill.key]
    indicators.push({ name: skill.label, max: 100 })
    values.push(data?.score || 0)
  })

  radarChart.setOption({
    tooltip: {
      trigger: 'item'
    },
    radar: {
      indicator: indicators,
      shape: 'circle',
      splitNumber: 5,
      axisName: {
        color: '#606266',
        fontSize: 12
      },
      splitLine: {
        lineStyle: { color: '#e4e7ed' }
      },
      splitArea: {
        areaStyle: {
          color: ['rgba(64, 158, 255, 0.05)', 'rgba(64, 158, 255, 0.1)']
        }
      }
    },
    series: [{
      type: 'radar',
      data: [{
        value: values,
        name: '技能评分',
        areaStyle: {
          color: 'rgba(64, 158, 255, 0.2)'
        },
        lineStyle: {
          color: '#409eff',
          width: 2
        },
        itemStyle: {
          color: '#409eff'
        }
      }]
    }]
  })
}

function handleResize() {
  radarChart && radarChart.resize()
}

async function loadStats() {
  try {
    const res = await practiceApi.getStats()
    if (res.data) {
      stats.value = res.data
      await nextTick()
      initRadarChart()
    }
  } catch (e) {
    console.error('加载实操统计失败:', e)
  }
}

onMounted(() => {
  loadStats()
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
  if (radarChart) { radarChart.dispose(); radarChart = null }
})
</script>

<style scoped>
.practice-report {
  position: absolute;
  top: 70px;
  right: 12px;
  width: 340px;
  max-height: calc(100% - 160px);
  background: rgba(255, 255, 255, 0.97);
  border-radius: 8px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.15);
  z-index: 10;
  overflow-y: auto;
}

.report-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  border-bottom: 1px solid #ebeef5;
}

.report-header h3 {
  font-size: 15px;
  color: #303133;
  margin: 0;
}

.report-body {
  padding: 16px;
}

.score-summary {
  display: flex;
  align-items: center;
  gap: 20px;
  margin-bottom: 20px;
}

.score-circle {
  width: 80px;
  height: 80px;
  border-radius: 50%;
  border: 4px solid var(--score-color, #409eff);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.score-value {
  font-size: 22px;
  font-weight: bold;
  color: var(--score-color, #409eff);
  line-height: 1;
}

.score-label {
  font-size: 11px;
  color: #909399;
  margin-top: 2px;
}

.summary-stats {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.summary-item {
  display: flex;
  align-items: baseline;
  gap: 6px;
}

.summary-value {
  font-size: 16px;
  font-weight: 600;
  color: #303133;
}

.summary-label {
  font-size: 12px;
  color: #909399;
}

.radar-section {
  margin-bottom: 20px;
}

.radar-section h4,
.type-section h4,
.difficulty-section h4 {
  font-size: 14px;
  color: #303133;
  margin: 0 0 10px;
}

.radar-chart {
  width: 100%;
  height: 220px;
}

.type-section {
  margin-bottom: 20px;
}

.type-bars {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.type-bar-item {
  padding: 4px 0;
}

.type-bar-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 4px;
}

.type-name {
  font-size: 13px;
  color: #303133;
  font-weight: 500;
}

.type-score {
  font-size: 13px;
  font-weight: 600;
  color: #409eff;
}

.type-count {
  font-size: 11px;
  color: #909399;
  margin-top: 2px;
}

.difficulty-section {
  margin-bottom: 10px;
}

.difficulty-bars {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.difficulty-item {
  display: flex;
  align-items: center;
  gap: 8px;
}

.diff-label {
  font-size: 13px;
  color: #606266;
  width: 36px;
  flex-shrink: 0;
}

.diff-score {
  font-size: 12px;
  color: #909399;
  width: 90px;
  text-align: right;
  flex-shrink: 0;
}

@media screen and (max-width: 768px) {
  .practice-report {
    top: auto;
    bottom: 50px;
    right: 8px;
    left: 8px;
    width: auto;
    max-height: 50%;
  }
}
</style>
