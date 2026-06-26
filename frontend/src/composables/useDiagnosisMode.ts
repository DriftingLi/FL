import { ref } from 'vue'
import { getDiagnosisScenarios, highlightPart, unhighlightPart } from '@/utils/forkliftModel'
import { ElMessage } from 'element-plus'

export function useDiagnosisMode(getPartMeshes) {
  const diagnosisAttempts = ref(0)
  const diagnosisScore = ref(0)
  const diagnosisHint = ref('')
  const currentDiagnosis = ref(null)
  const flashingPartId = ref(null)
  const practiceCompleted = ref(false)
  const practiceDuration = ref(0)
  let practiceStartTime = 0

  function startDiagnosis() {
    const scenarios = getDiagnosisScenarios()
    const scenario = scenarios[Math.floor(Math.random() * scenarios.length)]
    currentDiagnosis.value = scenario
    diagnosisHint.value = scenario.hint
    diagnosisAttempts.value = 0
    practiceStartTime = Date.now()
    practiceCompleted.value = false
    flashingPartId.value = scenario.partId
    return scenario.partId
  }

  function handleDiagnosisClick(partId) {
    if (practiceCompleted.value) return { completed: false }

    const partMeshes = getPartMeshes()
    diagnosisAttempts.value++

    if (partId === currentDiagnosis.value.partId) {
      practiceCompleted.value = true
      practiceDuration.value = Math.round((Date.now() - practiceStartTime) / 1000)
      flashingPartId.value = null

      if (partMeshes[partId]) {
        unhighlightPart(partMeshes[partId])
        highlightPart(partMeshes[partId], '#00ff88', 0.5)
      }

      if (diagnosisAttempts.value === 1) diagnosisScore.value = 100
      else if (diagnosisAttempts.value === 2) diagnosisScore.value = 80
      else if (diagnosisAttempts.value === 3) diagnosisScore.value = 60
      else diagnosisScore.value = Math.max(20, 100 - (diagnosisAttempts.value - 1) * 20)

      ElMessage.success('诊断正确！')
      return {
        completed: true,
        score: diagnosisScore.value,
        duration: practiceDuration.value,
        attempts: diagnosisAttempts.value
      }
    } else {
      ElMessage.warning('这不是故障部件，请继续排查')
      return { completed: false, wrongPart: true }
    }
  }

  function resetDiagnosis() {
    diagnosisAttempts.value = 0
    diagnosisScore.value = 0
    diagnosisHint.value = ''
    currentDiagnosis.value = null
    flashingPartId.value = null
    practiceCompleted.value = false
    practiceDuration.value = 0
  }

  return {
    diagnosisAttempts,
    diagnosisScore,
    diagnosisHint,
    currentDiagnosis,
    flashingPartId,
    practiceCompleted,
    practiceDuration,
    startDiagnosis,
    handleDiagnosisClick,
    resetDiagnosis
  }
}
