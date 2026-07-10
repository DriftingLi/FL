<template>
  <div class="practice-stats">
    <h2>练习统计</h2>
    <el-row :gutter="20" class="overview">
      <el-col :xs="12" :sm="6">
        <el-card class="stat-card">
          <div class="stat-num">{{ stats.total }}</div>
          <div class="stat-label">总练习数</div>
        </el-card>
      </el-col>
      <el-col :xs="12" :sm="6">
        <el-card class="stat-card correct">
          <div class="stat-num">{{ stats.correct }}</div>
          <div class="stat-label">正确数</div>
        </el-card>
      </el-col>
      <el-col :xs="12" :sm="6">
        <el-card class="stat-card wrong">
          <div class="stat-num">{{ stats.wrong }}</div>
          <div class="stat-label">错误数</div>
        </el-card>
      </el-col>
      <el-col :xs="12" :sm="6">
        <el-card class="stat-card rate">
          <div class="stat-num">{{ stats.accuracy }}%</div>
          <div class="stat-label">正确率</div>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="20">
      <el-col :xs="24" :md="12">
        <el-card>
          <h3>各题型正确率</h3>
          <div v-for="(item, type) in stats.by_type" :key="type" class="type-bar">
            <span class="type-label">{{ typeMap[type] || type }}</span>
            <el-progress :percentage="item.accuracy" :color="item.accuracy >= 70 ? '#67c23a' : '#f56c6c'" :stroke-width="18" :text-inside="true" />
            <span class="type-count">{{ item.correct }}/{{ item.total }}</span>
          </div>
        </el-card>
      </el-col>
      <el-col :xs="24" :md="12">
        <el-card>
          <h3>薄弱知识点</h3>
          <div v-if="stats.weak_knowledge_points && stats.weak_knowledge_points.length > 0">
            <div v-for="point in stats.weak_knowledge_points" :key="point.id" class="weak-item">
              <span class="weak-name">{{ point.name }}</span>
              <el-progress :percentage="point.accuracy" color="#f56c6c" :stroke-width="14" />
              <span class="weak-count">{{ point.accuracy }}%</span>
            </div>
          </div>
          <el-empty v-else description="暂无薄弱项" :image-size="60" />
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { practiceModeApi } from '@/api/practiceMode'

const typeMap = { single_choice: '单选题', multi_choice: '多选题', true_false: '判断题', fault_image: '故障识图', short_answer: '简答题' }

const stats = ref({ total: 0, correct: 0, wrong: 0, accuracy: 0, by_type: {} as Record<string, any>, by_level: {} as Record<string, any>, weak_knowledge_points: [] })

onMounted(async () => {
  try {
    const res = await practiceModeApi.getStats()
    stats.value = res.data || stats.value
  } catch (e) {}
})
</script>

<style scoped>
.practice-stats { max-width: 1200px; margin: 0 auto; }
.practice-stats h2 { margin-bottom: 20px; }
.overview { margin-bottom: 20px; }
.stat-card { text-align: center; margin-bottom: 15px; }
.stat-num { font-size: 32px; font-weight: bold; color: #409eff; }
.stat-card.correct .stat-num { color: #67c23a; }
.stat-card.wrong .stat-num { color: #f56c6c; }
.stat-card.rate .stat-num { color: #e6a23c; }
.stat-label { color: #909399; margin-top: 5px; }
.type-bar { display: flex; align-items: center; gap: 10px; margin-bottom: 12px; }
.type-label { width: 70px; font-size: 14px; }
.type-bar .el-progress { flex: 1; }
.type-count { width: 60px; text-align: right; font-size: 13px; color: #909399; }
.weak-item { display: flex; align-items: center; gap: 10px; margin-bottom: 10px; }
.weak-name { width: 100px; font-size: 14px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.weak-item .el-progress { flex: 1; }
.weak-count { width: 50px; text-align: right; font-size: 13px; color: #f56c6c; }
</style>
