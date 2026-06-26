import { ref } from 'vue'
import { practiceApi } from '@/api/practice'
import { getPartInfo } from '@/utils/forkliftModel'

export function usePracticeRecorder() {
  const operations = ref([])
  const isSaving = ref(false)

  function recordOperation(partId, action) {
    const partInfo = getPartInfo(partId)
    operations.value.push({
      partId,
      partName: partInfo?.name || partId,
      action,
      timestamp: Date.now()
    })
  }

  function clearOperations() {
    operations.value = []
  }

  async function savePracticeRecord(type, duration, score, difficulty = 'normal', extraData = {}) {
    isSaving.value = true
    try {
      const allOps = [...operations.value]
      await practiceApi.saveRecord({
        practice_type: type,
        duration: duration,
        score: score,
        operations: allOps,
        status: 'completed',
        difficulty: difficulty,
        scenario_id: extraData.scenarioId || null,
        time_limit: extraData.timeLimit || null,
        correct_parts: extraData.correctParts || null,
        wrong_attempts: extraData.wrongAttempts || 0
      })
      operations.value = []
      return true
    } catch (e) {
      console.error('保存实操记录失败:', e)
      return false
    } finally {
      isSaving.value = false
    }
  }

  return {
    operations,
    isSaving,
    recordOperation,
    clearOperations,
    savePracticeRecord
  }
}
