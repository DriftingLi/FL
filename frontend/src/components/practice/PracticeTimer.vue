<template>
  <div v-if="timeLimit > 0" class="practice-timer" :class="{ 'timer-warning': remaining <= 30, 'timer-danger': remaining <= 10 }">
    <el-icon :size="16"><Clock /></el-icon>
    <span class="timer-text">{{ formattedTime }}</span>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onBeforeUnmount } from 'vue'
import { Clock } from '@element-plus/icons-vue'

const props = defineProps({
  timeLimit: { type: Number, default: 0 },
  running: { type: Boolean, default: false }
})

const emit = defineEmits(['timeout'])

const remaining = ref(props.timeLimit)
let timerInterval = null

const formattedTime = computed(() => {
  const mins = Math.floor(remaining.value / 60)
  const secs = remaining.value % 60
  return `${mins}:${secs.toString().padStart(2, '0')}`
})

function startTimer() {
  stopTimer()
  remaining.value = props.timeLimit
  if (props.timeLimit <= 0) return

  timerInterval = setInterval(() => {
    if (remaining.value > 0) {
      remaining.value--
      if (remaining.value <= 0) {
        stopTimer()
        emit('timeout')
      }
    }
  }, 1000)
}

function stopTimer() {
  if (timerInterval) {
    clearInterval(timerInterval)
    timerInterval = null
  }
}

watch(() => props.running, (val) => {
  if (val) {
    startTimer()
  } else {
    stopTimer()
  }
}, { immediate: true })

watch(() => props.timeLimit, (val) => {
  remaining.value = val
  if (props.running) {
    startTimer()
  }
})

onBeforeUnmount(() => {
  stopTimer()
})

defineExpose({ startTimer, stopTimer, remaining })
</script>

<style scoped>
.practice-timer {
  position: absolute;
  top: 70px;
  left: 50%;
  transform: translateX(-50%);
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 16px;
  background: rgba(255, 255, 255, 0.95);
  border-radius: 20px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.15);
  z-index: 10;
  font-size: 16px;
  font-weight: 600;
  color: #303133;
}

.timer-warning {
  color: #E6A23C;
  background: rgba(230, 162, 60, 0.1);
  border: 1px solid #E6A23C;
}

.timer-danger {
  color: #F56C6C;
  background: rgba(245, 108, 108, 0.1);
  border: 1px solid #F56C6C;
  animation: pulse 1s ease-in-out infinite;
}

.timer-text {
  font-variant-numeric: tabular-nums;
  min-width: 42px;
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.6; }
}
</style>
