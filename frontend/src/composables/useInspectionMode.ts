import { ref, computed } from 'vue'
import { getPartInfo, getInspectionOrder, highlightPart, unhighlightPart } from '@/utils/forkliftModel'
import { ElMessage } from 'element-plus'

export function useInspectionMode(getPartMeshes) {
  const inspectionStep = ref(0)
  const inspectionSteps = ref([])
  const practiceCompleted = ref(false)
  const practiceDuration = ref(0)
  const practiceStartTime = ref(0)
  const completedParts = ref([])

  const currentInspectionPart = computed(() => {
    if (inspectionStep.value < inspectionSteps.value.length) {
      return inspectionSteps.value[inspectionStep.value]
    }
    return null
  })

  function startInspection() {
    const order = getInspectionOrder()
    inspectionSteps.value = order.map(partId => getPartInfo(partId)).filter(Boolean)
    inspectionStep.value = 0
    practiceStartTime.value = Date.now()
    practiceCompleted.value = false
    completedParts.value = []

    if (inspectionSteps.value.length > 0) {
      return inspectionSteps.value[0].partId
    }
    return null
  }

  function handleInspectionClick(partId) {
    if (practiceCompleted.value) return { completed: false, nextHighlight: null }

    const partMeshes = getPartMeshes()
    const expectedPartId = inspectionSteps.value[inspectionStep.value]?.partId

    if (partId === expectedPartId) {
      if (partMeshes[partId]) {
        unhighlightPart(partMeshes[partId])
        highlightPart(partMeshes[partId], '#00ff88', 0.5)
      }

      completedParts.value.push(partId)
      inspectionStep.value++

      if (inspectionStep.value >= inspectionSteps.value.length) {
        practiceCompleted.value = true
        practiceDuration.value = Math.round((Date.now() - practiceStartTime.value) / 1000)
        ElMessage.success('日常检查完成！')
        return { completed: true, nextHighlight: null, duration: practiceDuration.value }
      } else {
        const nextPartId = inspectionSteps.value[inspectionStep.value].partId
        return { completed: false, nextHighlight: nextPartId }
      }
    } else {
      ElMessage.warning('请先检查当前高亮的部件')
      return { completed: false, nextHighlight: null, wrongPart: true }
    }
  }

  function undoInspectionStep() {
    if (inspectionStep.value <= 0 || practiceCompleted.value) return null

    inspectionStep.value--
    const prevPartId = completedParts.value.pop()

    const partMeshes = getPartMeshes()
    if (prevPartId && partMeshes[prevPartId]) {
      unhighlightPart(partMeshes[prevPartId])
    }

    return inspectionSteps.value[inspectionStep.value]?.partId || null
  }

  function resetInspection() {
    inspectionStep.value = 0
    practiceCompleted.value = false
    practiceDuration.value = 0
    practiceStartTime.value = 0
    completedParts.value = []
  }

  return {
    inspectionStep,
    inspectionSteps,
    currentInspectionPart,
    practiceCompleted,
    practiceDuration,
    startInspection,
    handleInspectionClick,
    undoInspectionStep,
    resetInspection
  }
}
