<template>
  <div class="result-card">
    <div class="result-header">
      <span>正确答案是 <strong class="correct">{{ correctAnswer }}</strong>，你的答案是 <strong :class="isCorrect ? 'correct' : 'wrong'">{{ displayUserAnswer }}</strong></span>
    </div>
    <div class="result-stats">
      <div class="stat-item">
        <span class="stat-label">个人答题用时</span>
        <span class="stat-value" :style="{ color: '#67c23a' }">{{ durationText }}</span>
      </div>
      <div class="stat-item">
        <span class="stat-label">全站正确率</span>
        <span class="stat-value" :style="{ color: '#67c23a' }">{{ accuracyText }}</span>
      </div>
      <div class="stat-item">
        <span class="stat-label">易错项</span>
        <span class="stat-value" :style="{ color: commonWrong ? '#f56c6c' : '#909399' }">{{ commonWrong || '—' }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  correctAnswer: string
  userAnswer: unknown
  isCorrect?: boolean | null
  durationSeconds?: number
  accuracyRate?: number | null
  commonWrong?: string | null
  questionType?: string
}>()

const displayUserAnswer = computed(() => {
  if (props.userAnswer == null || props.userAnswer === '') return '未作答'
  if (Array.isArray(props.userAnswer)) return props.userAnswer.join('、') || '未作答'
  return String(props.userAnswer)
})

const durationText = computed(() => {
  if (props.durationSeconds == null) return '—'
  return `${props.durationSeconds.toFixed(1)}秒`
})

const accuracyText = computed(() => {
  if (props.accuracyRate == null || props.accuracyRate === undefined) return '—'
  return `${props.accuracyRate}%`
})
</script>

<style scoped>
.result-card {
  border: 1px solid #ebeef5;
  border-radius: 8px;
  padding: 16px;
  background: #fff;
  margin-bottom: 12px;
}
.result-header { font-size: 14px; color: #303133; margin-bottom: 16px; }
.result-header .correct { color: #67c23a; }
.result-header .wrong { color: #f56c6c; }
.result-stats { display: flex; justify-content: space-between; text-align: center; }
.stat-item { flex: 1; display: flex; flex-direction: column; gap: 4px; }
.stat-label { font-size: 12px; color: #909399; }
.stat-value { font-size: 15px; font-weight: 600; }
</style>
