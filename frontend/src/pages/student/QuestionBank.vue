<template>
  <div class="question-bank">
    <el-row :gutter="20">
      <el-col :span="24">
        <h2>题库练习</h2>
        <div class="user-level-badge">
          <el-tag :type="levelTagType" size="large" effect="dark">
            当前等级：{{ levelLabelMap[userLevel] }}学徒
          </el-tag>
          <span class="level-hint">{{ levelHint }}</span>
        </div>
      </el-col>
    </el-row>

    <el-row :gutter="20" class="level-cards">
      <el-col :xs="24" :sm="8" v-for="level in visibleLevels" :key="level.value">
        <el-card
          class="level-card"
          :class="['level-' + level.value, { 'level-locked': level.locked }]"
          shadow="hover"
          @click="!level.locked && selectLevel(level.value)"
        >
          <div class="level-icon">{{ level.icon }}</div>
          <h3>{{ level.label }}</h3>
          <p>{{ level.desc }}</p>
          <div class="level-stats">
            <span>{{ level.count }}道题</span>
          </div>
          <div v-if="level.locked" class="level-lock">
            <el-icon :size="20"><Lock /></el-icon>
            <span>未解锁</span>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="20" class="quick-links" v-if="userLevel !== 'expert'">
      <el-col :span="24">
        <el-button type="warning" size="large" @click="$router.push('/training/wrong-questions')" style="width:100%">
          错题本
        </el-button>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { Lock } from '@element-plus/icons-vue'
import { questionBankApi } from '@/api/questionBank'
import { useAuthStore } from '@/stores/auth'
import { useUserStore } from '@/stores/user'
const authStore = useAuthStore()
const userStore = useUserStore()

const levelLabelMap = { beginner: '初级', intermediate: '中级', advanced: '高级', expert: '顶级' }
const levelOrder = { beginner: 1, intermediate: 2, advanced: 3 }
const levelQuestionCount = { beginner: 10, intermediate: 20, advanced: 30 }
const levelAllowedLevels = {
  beginner: ['beginner'],
  intermediate: ['beginner', 'intermediate'],
  advanced: ['beginner', 'intermediate', 'advanced']
}

const userLevel = ref(authStore.userInfo?.level || 'beginner')

const levelTagType = computed(() => {
  const map = { beginner: 'success', intermediate: 'warning', advanced: 'danger', expert: '' }
  return map[userLevel.value] || 'success'
})

const levelHint = computed(() => {
  if (userLevel.value === 'expert') return '您已达到最高等级，无需刷题'
  const hints = {
    beginner: '刷题范围：初级题库，每次10道',
    intermediate: '刷题范围：初级+中级题库，每次20道',
    advanced: '刷题范围：全部题库，每次30道'
  }
  return hints[userLevel.value] || hints.beginner
})

const questionCount = computed(() => levelQuestionCount[userLevel.value] || 10)

const levels = ref([
  { value: 'beginner', label: '初级学徒', icon: '🟢', desc: '叉车基础操作与安全规范', count: 0 },
  { value: 'intermediate', label: '中级学徒', icon: '🟡', desc: '故障诊断与维修技能', count: 0 },
  { value: 'advanced', label: '高级学徒', icon: '🔴', desc: '高级维修与教学能力', count: 0 }
])

const visibleLevels = computed(() => {
  const allowed = levelAllowedLevels[userLevel.value] || ['beginner']
  return levels.value.map(l => ({
    ...l,
    locked: !allowed.includes(l.value)
  }))
})

const selectedLevel = ref('beginner')

onMounted(async () => {
  if (!authStore.userInfo?.level) {
    try {
      await userStore.fetchProfile()
      userLevel.value = userStore.profile?.level || 'beginner'
    } catch (e) {}
  }

  const allowed = levelAllowedLevels[userLevel.value] || ['beginner']
  selectedLevel.value = allowed[allowed.length - 1]

  try {
    const res = await questionBankApi.getStats()
    if (res.data) {
      const stats = res.data
      levels.value[0].count = stats.by_level?.beginner || 0
      levels.value[1].count = stats.by_level?.intermediate || 0
      levels.value[2].count = stats.by_level?.advanced || 0
    }
  } catch (e) {}
})

function selectLevel(level) {
  selectedLevel.value = level
}
</script>

<style scoped>
.question-bank { max-width: 1200px; margin: 0 auto; }
.question-bank h2 { margin-bottom: 10px; color: #303133; }
.user-level-badge { display: flex; align-items: center; gap: 12px; margin-bottom: 20px; }
.level-hint { color: #909399; font-size: 14px; }
.level-cards { margin-bottom: 30px; }
.level-card { cursor: pointer; text-align: center; transition: transform 0.3s; margin-bottom: 15px; position: relative; }
.level-card:hover { transform: translateY(-5px); }
.level-card.level-locked { cursor: not-allowed; opacity: 0.6; }
.level-card.level-locked:hover { transform: none; }
.level-icon { font-size: 48px; margin-bottom: 10px; }
.level-card h3 { margin: 10px 0; }
.level-card p { color: #909399; font-size: 14px; }
.level-stats { margin-top: 10px; color: #409eff; font-weight: bold; }
.level-lock { position: absolute; top: 10px; right: 10px; display: flex; align-items: center; gap: 4px; color: #909399; font-size: 12px; }
.level-beginner { border-top: 3px solid #67c23a; }
.level-intermediate { border-top: 3px solid #e6a23c; }
.level-advanced { border-top: 3px solid #f56c6c; }
.mode-section { margin-bottom: 30px; }
.mode-card { cursor: pointer; text-align: center; padding: 20px; transition: transform 0.3s; margin-bottom: 15px; }
.mode-card:hover { transform: translateY(-3px); }
.mode-card h4 { margin: 10px 0 5px; }
.mode-card p { color: #909399; font-size: 13px; }
.quick-links { margin-top: 20px; }
.quick-links .el-col { margin-bottom: 10px; }
</style>
