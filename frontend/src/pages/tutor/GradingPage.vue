<template>
  <div class="grading-page">
    <h2>考试批改</h2>

    <el-card class="stats-card">
      <el-row :gutter="20">
        <el-col :span="8">
          <el-statistic title="待批改学员" :value="pendingCount" />
        </el-col>
        <el-col :span="8">
          <el-statistic title="已批改学员" :value="completedCount" />
        </el-col>
        <el-col :span="8">
          <el-statistic title="总提交数" :value="participants.length" />
        </el-col>
      </el-row>
    </el-card>

    <div v-if="!selectedParticipant" class="participant-list">
      <el-table :data="participants" stripe v-loading="loading" @row-click="openDetail">
        <el-table-column prop="session_name" label="考试名称" min-width="150" />
        <el-table-column prop="session_level" label="等级" width="80">
          <template #default="{ row }">{{ levelMap[row.session_level] }}</template>
        </el-table-column>
        <el-table-column prop="student_name" label="学员" width="100" />
        <el-table-column label="得分" width="100">
          <template #default="{ row }">
            <span v-if="row.score !== null">{{ row.score }}分</span>
            <span v-else style="color:#909399">待批改</span>
          </template>
        </el-table-column>
        <el-table-column label="批改进度" width="140">
          <template #default="{ row }">
            <el-progress
              :percentage="Math.round((row.total_answers - row.ungraded_count) / row.total_answers * 100)"
              :status="row.ungraded_count === 0 ? 'success' : ''"
            />
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag v-if="row.grading_status === 'completed'" type="success" size="small">已完成</el-tag>
            <el-tag v-else type="warning" size="small">待批改</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="100">
          <template #default="{ row }">
            <el-button type="primary" size="small" @click.stop="openDetail(row)">批改</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <div v-if="selectedParticipant" class="detail-view">
      <div class="detail-header">
        <el-button @click="closeDetail" :icon="ArrowLeft">返回列表</el-button>
        <div class="detail-info">
          <span><strong>{{ detail.session_name }}</strong></span>
          <el-tag size="small">{{ levelMap[detail.session_level] }}</el-tag>
          <span>学员：{{ detail.student_name }}</span>
          <span>得分：{{ detail.score !== null ? detail.score + '分' : '待批改' }} / {{ detail.pass_score }}分及格</span>
          <el-tag v-if="detail.is_passed" type="success" size="small">通过</el-tag>
          <el-tag v-else-if="detail.score !== null" type="danger" size="small">未通过</el-tag>
        </div>
        <el-button
          v-if="detail.objective_ungraded > 0"
          type="primary"
          @click="confirmAllObjective"
          :loading="confirmingObj"
        >
          一键确认全部客观题（{{ detail.objective_ungraded }}道）
        </el-button>
      </div>

      <div class="answer-list">
        <el-card v-for="(ans, idx) in detail.answers" :key="ans.id" class="answer-item">
          <div class="answer-header">
            <span class="answer-index">第{{ idx + 1 }}题</span>
            <el-tag size="small">{{ typeMap[ans.question?.type] }}</el-tag>
            <span class="answer-score">
              得分：<strong>{{ ans.score }}</strong> / {{ ans.question?.score || 0 }}分
            </span>
            <el-tag v-if="ans.grader_id" type="success" size="small">已批改</el-tag>
            <el-tag v-else type="warning" size="small">待批改</el-tag>
          </div>

          <div class="answer-body">
            <div class="question-section">
              <p class="q-content">{{ ans.question?.content }}</p>
              <img v-if="ans.question?.image_url" :src="ans.question.image_url" class="q-image" />
              <p v-if="ans.question?.reference_answer" class="ref-answer">
                <strong>参考答案：</strong>{{ ans.question.reference_answer }}
              </p>
              <p v-if="ans.question?.scoring_criteria" class="scoring-criteria">
                <strong>评分标准：</strong>{{ ans.question.scoring_criteria }}
              </p>
            </div>

            <div class="student-answer-section">
              <p><strong>学员答案：</strong>{{ ans.user_answer || '未作答' }}</p>
            </div>

            <div v-if="ans.question?.type !== 'short_answer' && !ans.grader_id" class="objective-confirm">
              <span class="auto-result">
                系统自动评分：<strong>{{ ans.score }}</strong>分
                <el-tag v-if="ans.is_correct" type="success" size="small">正确</el-tag>
                <el-tag v-else type="danger" size="small">错误</el-tag>
              </span>
            </div>

            <div v-if="ans.question?.type === 'short_answer' && !ans.grader_id" class="subjective-grading">
              <div v-if="ans.ai_score != null" class="ai-grading-box">
                <div class="ai-grading-header">
                  <el-icon><Monitor /></el-icon>
                  <span>AI 建议评分</span>
                </div>
                <div class="ai-grading-content">
                  <p class="ai-score-text">
                    <strong>{{ ans.ai_score }}</strong> / {{ ans.question?.score || 10 }}分
                  </p>
                  <p v-if="ans.ai_comment" class="ai-comment-text">{{ ans.ai_comment }}</p>
                </div>
                <div class="ai-grading-actions">
                  <el-button type="success" size="small" @click="confirmAi(ans)" :loading="ans._confirming">
                    确认AI评分
                  </el-button>
                  <span class="or-divider">或手动评分</span>
                </div>
              </div>
              <div v-else class="ai-trigger-box">
                <el-button type="primary" plain size="small" @click="triggerAi(ans)" :loading="ans._aiLoading">
                  AI智能评分
                </el-button>
                <span class="or-divider">或手动评分</span>
              </div>
              <div class="manual-grading-form">
                <el-input-number v-model="ans._score" :min="0" :max="ans.question?.score || 10" :step="0.5" size="small" />
                <span style="margin:0 8px">/ {{ ans.question?.score || 10 }}分</span>
                <el-input v-model="ans._comment" placeholder="评语（可选）" style="width:250px" size="small" />
                <el-button type="primary" size="small" @click="gradeAnswer(ans)" style="margin-left:8px">评分</el-button>
              </div>
            </div>

            <div v-if="ans.grader_id" class="graded-info">
              <span v-if="ans.grading_comment" class="grading-comment">评语：{{ ans.grading_comment }}</span>
              <el-button type="primary" plain size="small" @click="startRegrade(ans)">复核</el-button>
            </div>
            <div v-if="ans._regrading" class="regrade-form">
              <el-input-number v-model="ans._regradeScore" :min="0" :max="ans.question?.score || 10" :step="0.5" size="small" />
              <span style="margin:0 8px">/ {{ ans.question?.score || 10 }}分</span>
              <el-input v-model="ans._regradeComment" placeholder="复核评语（可选）" style="width:250px" size="small" />
              <el-button type="primary" size="small" @click="doRegrade(ans)">确认复核</el-button>
              <el-button size="small" @click="ans._regrading = false">取消</el-button>
            </div>
          </div>
        </el-card>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Monitor, ArrowLeft } from '@element-plus/icons-vue'
import { gradingApi } from '@/api/grading'

const levelMap = { beginner: '初级', intermediate: '中级', advanced: '高级', expert: '顶级' }
const typeMap = { single_choice: '单选题', multi_choice: '多选题', true_false: '判断题', fault_image: '故障识图', short_answer: '简答题' }

const loading = ref(false)
const participants = ref([])
const selectedParticipant = ref(null)
const detail = ref({})
const confirmingObj = ref(false)

const pendingCount = computed(() => participants.value.filter(p => p.grading_status === 'pending').length)
const completedCount = computed(() => participants.value.filter(p => p.grading_status === 'completed').length)

onMounted(() => { loadParticipants() })

async function loadParticipants() {
  loading.value = true
  try {
    const res = await gradingApi.getSubmittedParticipants()
    participants.value = res.data || []
  } catch (e) {} finally { loading.value = false }
}

async function openDetail(row) {
  try {
    const res = await gradingApi.getParticipantDetail(row.id)
    const data = res.data || {}
    if (data.answers) {
      data.answers.forEach(a => {
        a._score = a.ai_score != null ? a.ai_score : 0
        a._comment = ''
        a._confirming = false
        a._aiLoading = false
        a._regrading = false
        a._regradeScore = a.score || 0
        a._regradeComment = ''
      })
    }
    detail.value = data
    selectedParticipant.value = row.id
  } catch (e) {
    ElMessage.error('加载详情失败')
  }
}

function closeDetail() {
  selectedParticipant.value = null
  detail.value = {}
  loadParticipants()
}

async function confirmAllObjective() {
  try {
    await ElMessageBox.confirm(
      '确认全部客观题自动批改结果？确认后客观题评分将锁定。',
      '确认客观题批改',
      { confirmButtonText: '确认', cancelButtonText: '取消', type: 'info' }
    )
    confirmingObj.value = true
    await gradingApi.confirmObjectiveAnswers(selectedParticipant.value)
    ElMessage.success('客观题批改确认成功')
    await openDetail({ id: selectedParticipant.value })
  } catch (e) {
    if (e !== 'cancel') ElMessage.error(e.message || '确认失败')
  } finally { confirmingObj.value = false }
}

async function confirmAi(ans) {
  try {
    await ElMessageBox.confirm(
      `确认采用AI建议评分 ${ans.ai_score} 分？`,
      '确认AI评分',
      { confirmButtonText: '确认', cancelButtonText: '取消', type: 'info' }
    )
    ans._confirming = true
    await gradingApi.confirmAiGrading(ans.id)
    ElMessage.success('AI评分确认成功')
    await openDetail({ id: selectedParticipant.value })
  } catch (e) {
    if (e !== 'cancel') ElMessage.error(e.message || '确认失败')
  } finally { ans._confirming = false }
}

async function triggerAi(ans) {
  try {
    ans._aiLoading = true
    const res = await gradingApi.aiGradeAnswer(ans.id)
    if (res.data) {
      ans.ai_score = res.data.ai_score
      ans.ai_comment = res.data.ai_comment
      ans._score = res.data.ai_score || 0
      ElMessage.success('AI评分完成')
    }
  } catch (e) {
    ElMessage.error(e.message || 'AI评分失败')
  } finally { ans._aiLoading = false }
}

async function gradeAnswer(ans) {
  try {
    await gradingApi.gradeAnswer(ans.id, { score: ans._score, comment: ans._comment })
    ElMessage.success('评分成功')
    await openDetail({ id: selectedParticipant.value })
  } catch (e) {
    ElMessage.error(e.message || '评分失败')
  }
}

function startRegrade(ans) {
  ans._regrading = true
  ans._regradeScore = ans.score || 0
  ans._regradeComment = ''
}

async function doRegrade(ans) {
  try {
    await gradingApi.regradeAnswer(ans.id, { score: ans._regradeScore, comment: ans._regradeComment })
    ElMessage.success('复核成功')
    await openDetail({ id: selectedParticipant.value })
  } catch (e) {
    ElMessage.error(e.message || '复核失败')
  }
}
</script>

<style scoped>
.grading-page h2 { margin-bottom: 20px; }
.stats-card { margin-bottom: 20px; }
.participant-list { margin-top: 15px; }

.detail-header {
  display: flex;
  align-items: center;
  gap: 15px;
  margin-bottom: 20px;
  padding: 12px 16px;
  background: #fff;
  border-radius: 8px;
  box-shadow: 0 2px 4px rgba(0,0,0,0.06);
  flex-wrap: wrap;
}
.detail-info { display: flex; align-items: center; gap: 10px; flex: 1; flex-wrap: wrap; }

.answer-item { margin-bottom: 12px; }
.answer-header { display: flex; gap: 10px; align-items: center; margin-bottom: 10px; }
.answer-index { font-weight: bold; font-size: 15px; }
.answer-score { margin-left: auto; }

.answer-body { padding: 0 4px; }
.question-section { margin-bottom: 10px; }
.q-content { font-size: 15px; line-height: 1.7; margin-bottom: 6px; }
.q-image { max-width: 100%; max-height: 200px; border-radius: 6px; margin-bottom: 6px; }
.ref-answer { color: #67c23a; font-size: 13px; background: #f0f9eb; padding: 6px 10px; border-radius: 4px; }
.scoring-criteria { color: #909399; font-size: 13px; margin-top: 4px; }

.student-answer-section {
  padding: 8px 12px;
  background: #f5f7fa;
  border-radius: 6px;
  margin-bottom: 10px;
}

.objective-confirm { margin-bottom: 8px; }
.auto-result { font-size: 14px; }

.subjective-grading { margin-top: 8px; }

.ai-grading-box {
  margin: 8px 0;
  padding: 10px 14px;
  background: linear-gradient(135deg, #f0f9eb 0%, #e1f3d8 100%);
  border: 1px solid #e1f3d8;
  border-radius: 8px;
}
.ai-grading-header { display: flex; align-items: center; gap: 6px; color: #67c23a; font-weight: 600; margin-bottom: 6px; }
.ai-grading-content { margin-bottom: 6px; }
.ai-score-text { font-size: 16px; color: #409eff; }
.ai-score-text strong { font-size: 20px; }
.ai-comment-text { color: #606266; font-size: 13px; margin-top: 4px; line-height: 1.5; }
.ai-grading-actions { display: flex; align-items: center; gap: 8px; }

.ai-trigger-box { margin: 8px 0; display: flex; align-items: center; gap: 8px; }

.manual-grading-form { display: flex; align-items: center; margin-top: 8px; flex-wrap: wrap; gap: 4px; }

.graded-info { display: flex; align-items: center; gap: 10px; margin-top: 6px; }
.grading-comment { color: #909399; font-size: 13px; }

.regrade-form { display: flex; align-items: center; margin-top: 8px; flex-wrap: wrap; gap: 4px; }

.or-divider { color: #909399; font-size: 12px; }
</style>
