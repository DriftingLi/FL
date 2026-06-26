<template>
  <div class="practice-page">
    <template v-if="!webglSupported">
      <WebGLFallback :visible="true" />
    </template>

    <div v-else class="practice-container" ref="practiceContainer" @contextmenu.prevent="onContainerContextMenu">
      <div ref="canvasContainer" class="canvas-container"></div>

      <PracticeToolbar
        :practiceMode="practiceMode"
        :lowQualityMode="lowQualityMode"
        :difficulty="difficulty"
        :soundEnabled="soundEnabled"
        @update:practiceMode="handleModeChange"
        @update:lowQualityMode="handleQualityChange"
        @update:difficulty="handleDifficultyChange"
        @update:soundEnabled="handleSoundToggle"
        @reset-camera="resetCamera"
        @toggle-fullscreen="toggleFullscreen"
        @take-screenshot="takeScreenshot"
        @show-replay="handleShowReplay"
        @show-report="showReport = !showReport"
      />

      <PartInfoSidebar
        :part="selectedPart"
        @close="clearSelection"
      />

      <InspectionPanel
        v-if="practiceMode === 'inspection'"
        :step="inspection.inspectionStep.value"
        :steps="inspection.inspectionSteps.value"
        :currentPart="inspection.currentInspectionPart.value"
        :completed="inspection.practiceCompleted.value"
        :duration="inspection.practiceDuration.value"
        @restart="restartPractice"
        @undo="handleInspectionUndo"
      />

      <DiagnosisPanel
        v-if="practiceMode === 'diagnosis'"
        :hint="diagnosis.diagnosisHint.value"
        :attempts="diagnosis.diagnosisAttempts.value"
        :completed="diagnosis.practiceCompleted.value"
        :score="diagnosis.diagnosisScore.value"
        :duration="diagnosis.practiceDuration.value"
        :fault="diagnosis.currentDiagnosis.value?.fault || ''"
        @restart="restartPractice"
      />

      <AssemblyPanel
        v-if="practiceMode === 'assembly'"
        :partStates="assemblyState.partStates"
        @detach="handleDetach"
        @attach="handleAttach"
        @reset="handleAssemblyReset"
      />

      <DemoPanel
        v-if="practiceMode === 'demo'"
        :animations="demoAnimList"
        :currentAnimation="currentDemoAnim"
        :isPlaying="isDemoPlaying"
        :progress="demoProgress"
        :currentTime="demoCurrentTime"
        :duration="demoDuration"
        :speed="demoSpeed"
        :narration="demoNarration"
        @select="handleDemoSelect"
        @play="handleDemoPlay"
        @pause="handleDemoPause"
        @stop="handleDemoStop"
        @seek="handleDemoSeek"
        @speed="handleDemoSpeed"
        @close="handleModeChange('free')"
      />

      <ReplayPanel
        v-if="showReplay"
        :operations="replayOperations"
        :currentStep="replayStep"
        :currentOp="replayCurrentOp"
        :replaying="replayActive"
        :isPlaying="isReplayPlaying"
        :replayProgress="replayProgress"
        :speed="replaySpeed"
        @close="stopReplay"
        @start-replay="startReplay"
        @stop-replay="stopReplay"
        @pause-replay="pauseReplay"
        @resume-replay="resumeReplay"
        @seek="seekReplay"
        @speed="setReplaySpeed"
      />

      <PracticeReport
        v-if="showReport"
        @close="showReport = false"
      />

      <PracticeTimer
        v-if="timerActive"
        :timeLimit="timerLimit"
        :running="timerRunning"
        @timeout="handleTimeout"
      />

      <OperationTips />

      <GuidedTour
        :visible="showTour"
        :currentStep="tourStep"
        @next="tourStep++"
        @prev="tourStep--"
        @skip="finishTour"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { ElMessage } from 'element-plus'
import { checkWebGL, highlightPart, unhighlightPart, flashPart, getPartInfo, showLabel, hideLabel, getDifficultySettings } from '@/utils/forkliftModel'
import { createAssemblyState, detachPart, attachPart, updateAssemblyAnimation, resetAssembly, canDetach, canAttach, getDetachBlockers, getAttachBlockers } from '@/utils/partAssembly'
import { createAnimationPlayer, getDemoAnimationList, getDemoAnimations } from '@/utils/animationPlayer'
import { createOperationReplayer } from '@/utils/operationReplayer'
import { playClick, playSuccess, playFail, playDetach, playAttach, setSoundEnabled, initAudio } from '@/utils/soundManager'
import { useThreeScene } from '@/composables/useThreeScene'
import { useInspectionMode } from '@/composables/useInspectionMode'
import { useDiagnosisMode } from '@/composables/useDiagnosisMode'
import { usePracticeRecorder } from '@/composables/usePracticeRecorder'
import PracticeToolbar from '@/components/practice/PracticeToolbar.vue'
import PartInfoSidebar from '@/components/practice/PartInfoSidebar.vue'
import InspectionPanel from '@/components/practice/InspectionPanel.vue'
import DiagnosisPanel from '@/components/practice/DiagnosisPanel.vue'
import AssemblyPanel from '@/components/practice/AssemblyPanel.vue'
import DemoPanel from '@/components/practice/DemoPanel.vue'
import ReplayPanel from '@/components/practice/ReplayPanel.vue'
import PracticeReport from '@/components/practice/PracticeReport.vue'
import PracticeTimer from '@/components/practice/PracticeTimer.vue'
import OperationTips from '@/components/practice/OperationTips.vue'
import WebGLFallback from '@/components/practice/WebGLFallback.vue'
import GuidedTour from '@/components/practice/GuidedTour.vue'

const webglSupported = ref(false)
const practiceContainer = ref(null)
const canvasContainer = ref(null)
const practiceMode = ref('free')
const lowQualityMode = ref(false)
const selectedPart = ref(null)
const showTour = ref(false)
const tourStep = ref(0)
const difficulty = ref('normal')
const timerActive = ref(false)
const timerLimit = ref(0)
const timerRunning = ref(false)
const soundEnabled = ref(true)

const demoAnimList = getDemoAnimationList()
const currentDemoAnim = ref(null)
const demoProgress = ref(0)
const demoCurrentTime = ref(0)
const demoDuration = ref(0)
const demoSpeed = ref(1)
const demoNarration = ref('')
const isDemoPlaying = ref(false)

const showReplay = ref(false)
const showReport = ref(false)
const replayOperations = ref([])
const replayStep = ref(0)
const replayCurrentOp = ref(null)
const replayActive = ref(false)
const isReplayPlaying = ref(false)
const replayProgress = ref(0)
const replaySpeed = ref(1)

let highlightedPartId = null

const sceneApi = useThreeScene(canvasContainer, lowQualityMode)
const getPartMeshes = () => sceneApi.partMeshes
const getRenderer = () => sceneApi.renderer

const inspection = useInspectionMode(getPartMeshes)
const diagnosis = useDiagnosisMode(getPartMeshes)
const recorder = usePracticeRecorder()
const assemblyState = createAssemblyState()
const animPlayer = createAnimationPlayer()
const replayer = createOperationReplayer()

replayer.onStep((data) => {
  replayStep.value = data.index
  replayCurrentOp.value = data.operation
  playClick()
})

replayer.onProgress((progress) => {
  replayProgress.value = progress
})

replayer.onComplete(() => {
  replayActive.value = false
  isReplayPlaying.value = false
  playSuccess()
  ElMessage.success('回放完成')
  if (sceneApi.controls) sceneApi.controls.enabled = true
})

animPlayer.onComplete(() => {
  playSuccess()
  ElMessage.success('演示播放完成')
  currentDemoAnim.value = null
  isDemoPlaying.value = false
  if (sceneApi.controls) sceneApi.controls.enabled = true
})

animPlayer.onNarration((text) => {
  demoNarration.value = text
})

function onModeAnimationCallback(elapsed, delta) {
  const partMeshes = getPartMeshes()

  if (diagnosis.flashingPartId.value && partMeshes[diagnosis.flashingPartId.value]) {
    const settings = getDifficultySettings(difficulty.value)
    if (settings.showHighlight || practiceMode.value !== 'diagnosis') {
      flashPart(partMeshes[diagnosis.flashingPartId.value], elapsed)
    }
  }

  if (highlightedPartId && partMeshes[highlightedPartId] && practiceMode.value === 'inspection') {
    const settings = getDifficultySettings(difficulty.value)
    if (settings.showHighlight) {
      const pulse = (Math.sin(elapsed * 4) + 1) / 2 * 0.3 + 0.2
      highlightPart(partMeshes[highlightedPartId], '#00ff88', pulse)
    }
  }

  if (practiceMode.value === 'assembly') {
    updateAssemblyAnimation(partMeshes, assemblyState)
  }

  if (practiceMode.value === 'demo' && animPlayer.isPlaying()) {
    animPlayer.update(
      delta || 0.016,
      sceneApi.camera,
      sceneApi.controls,
      partMeshes,
      highlightPart,
      () => Object.values(partMeshes).forEach(mesh => unhighlightPart(mesh))
    )
    const state = animPlayer.getState()
    demoProgress.value = state.progress * 100
    demoCurrentTime.value = state.currentTime
    isDemoPlaying.value = true
  } else if (practiceMode.value === 'demo') {
    isDemoPlaying.value = false
  }

  if (showReplay.value && replayer.isPlaying()) {
    replayer.update(
      delta || 0.016,
      partMeshes,
      sceneApi.camera,
      sceneApi.controls
    )
    isReplayPlaying.value = true
  } else if (showReplay.value) {
    isReplayPlaying.value = false
  }
}

function initScene() {
  sceneApi.initScene()

  const renderer = getRenderer()
  if (renderer) {
    renderer.domElement.addEventListener('pointerdown', onCanvasPointerDown)
    renderer.domElement.addEventListener('pointerup', onCanvasPointerUp)
    renderer.domElement.addEventListener('mousemove', onCanvasMouseMove)
    renderer.domElement.addEventListener('contextmenu', onCanvasContextMenu)
  }

  sceneApi.registerAnimationCallback(onModeAnimationCallback)
  initAudio()
}

let pointerDownPos = null

function onCanvasPointerDown(event) {
  if (event.button === 0) {
    pointerDownPos = { x: event.clientX, y: event.clientY }
  }
  if (event.button === 2) {
    event.preventDefault()
  }
}

function onCanvasPointerUp(event) {
  if (event.button !== 0 || !pointerDownPos) return

  const dx = event.clientX - pointerDownPos.x
  const dy = event.clientY - pointerDownPos.y
  const distance = Math.sqrt(dx * dx + dy * dy)
  pointerDownPos = null

  if (distance > 5) return

  if (practiceMode.value === 'demo' && animPlayer.isPlaying()) return
  if (showReplay.value && replayer.isPlaying()) return

  sceneApi.markInteraction()

  const intersects = sceneApi.getRaycasterIntersection(event.clientX, event.clientY)
  const partId = sceneApi.findPartIdFromIntersection(intersects)

  if (partId) {
    const partInfo = getPartInfo(partId)
    if (!partInfo) return

    playClick()
    recorder.recordOperation(partId, 'click')

    if (practiceMode.value === 'free' || practiceMode.value === 'demo') {
      selectPart(partId)
    } else if (practiceMode.value === 'inspection') {
      handleInspectionClick(partId)
    } else if (practiceMode.value === 'diagnosis') {
      handleDiagnosisClick(partId)
    } else if (practiceMode.value === 'assembly') {
      handleAssemblyClick(partId)
    }
  } else {
    if (practiceMode.value === 'free' || practiceMode.value === 'assembly' || practiceMode.value === 'demo') {
      clearSelection()
    }
  }
}

let hoveredPartId = null

function onCanvasMouseMove(event) {
  const intersects = sceneApi.getRaycasterIntersection(event.clientX, event.clientY)
  const partId = sceneApi.findPartIdFromIntersection(intersects)
  const renderer = getRenderer()
  const labels = sceneApi.partLabels

  if (partId && partId !== hoveredPartId) {
    if (hoveredPartId) hideLabel(labels, hoveredPartId)
    hoveredPartId = partId
    showLabel(labels, partId)
    if (renderer) renderer.domElement.style.cursor = 'pointer'
  } else if (!partId) {
    if (hoveredPartId) hideLabel(labels, hoveredPartId)
    hoveredPartId = null
    if (renderer) renderer.domElement.style.cursor = 'default'
  }
}

function onCanvasContextMenu(event) {
  event.preventDefault()
  event.stopPropagation()
}

function onContainerContextMenu(event) {
  event.preventDefault()
  event.stopPropagation()
}

function selectPart(partId) {
  const partMeshes = getPartMeshes()
  if (highlightedPartId && highlightedPartId !== partId && partMeshes[highlightedPartId]) {
    unhighlightPart(partMeshes[highlightedPartId])
  }
  highlightedPartId = partId
  if (partMeshes[partId]) highlightPart(partMeshes[partId], '#00aaff', 0.3)
  selectedPart.value = getPartInfo(partId)
}

function clearSelection() {
  const partMeshes = getPartMeshes()
  if (highlightedPartId && partMeshes[highlightedPartId]) {
    unhighlightPart(partMeshes[highlightedPartId])
  }
  highlightedPartId = null
  selectedPart.value = null
}

function handleModeChange(mode) {
  const prevMode = practiceMode.value
  practiceMode.value = mode
  clearSelection()
  stopTimer()
  stopReplay()
  const partMeshes = getPartMeshes()
  Object.values(partMeshes).forEach(mesh => unhighlightPart(mesh))

  if (prevMode === 'diagnosis') {
    diagnosis.resetDiagnosis()
  }
  if (prevMode === 'inspection') {
    inspection.resetInspection()
  }

  if (mode === 'inspection') {
    startInspection()
  } else if (mode === 'diagnosis') {
    startDiagnosis()
  } else if (mode === 'assembly') {
    const partMeshes = getPartMeshes()
    resetAssembly(partMeshes, assemblyState)
  } else if (mode === 'demo') {
    animPlayer.stop()
    currentDemoAnim.value = null
    demoNarration.value = ''
    demoProgress.value = 0
    demoCurrentTime.value = 0
  } else {
    if (sceneApi.controls) sceneApi.controls.enabled = true
  }
}

function handleDifficultyChange(level) {
  difficulty.value = level
  const settings = getDifficultySettings(level)
  if (practiceMode.value === 'inspection' || practiceMode.value === 'diagnosis') {
    if (settings.timeLimit > 0) {
      timerLimit.value = settings.timeLimit
      timerActive.value = true
      timerRunning.value = true
    } else {
      stopTimer()
    }
    if (practiceMode.value === 'inspection') startInspection()
    else if (practiceMode.value === 'diagnosis') startDiagnosis()
  }
}

function handleQualityChange(low) {
  lowQualityMode.value = low
  sceneApi.handleQualityChange(low)
}

function handleSoundToggle(enabled) {
  soundEnabled.value = enabled
  setSoundEnabled(enabled)
}

function handleDemoSelect(animId) {
  const anims = getDemoAnimations()
  const anim = anims[animId]
  if (!anim) return

  currentDemoAnim.value = { name: anim.name, description: anim.description, duration: anim.duration }
  demoDuration.value = anim.duration
  demoProgress.value = 0
  demoCurrentTime.value = 0
  demoNarration.value = ''
  isDemoPlaying.value = false

  animPlayer.load(animId)
  animPlayer.play()
  isDemoPlaying.value = true

  if (sceneApi.controls) sceneApi.controls.enabled = false
}

function handleDemoStop() {
  animPlayer.stop()
  currentDemoAnim.value = null
  demoNarration.value = ''
  demoProgress.value = 0
  demoCurrentTime.value = 0
  isDemoPlaying.value = false
  if (sceneApi.controls) sceneApi.controls.enabled = true
  resetCamera()
}

function handleDemoPlay() {
  animPlayer.play()
  isDemoPlaying.value = true
}

function handleDemoPause() {
  animPlayer.pause()
  isDemoPlaying.value = false
}

function handleDemoSeek(progress) {
  if (!currentDemoAnim.value) return
  const time = progress * demoDuration.value
  animPlayer.seek(time)
  demoCurrentTime.value = time
}

function handleDemoSpeed(speed) {
  demoSpeed.value = speed
  animPlayer.setSpeed(speed)
}

function startInspection() {
  inspection.resetInspection()
  const firstPartId = inspection.startInspection()
  highlightedPartId = firstPartId
  const settings = getDifficultySettings(difficulty.value)
  if (settings.timeLimit > 0) {
    timerLimit.value = settings.timeLimit
    timerActive.value = true
    timerRunning.value = true
  } else {
    stopTimer()
  }
}

function handleInspectionClick(partId) {
  const settings = getDifficultySettings(difficulty.value)
  const result = inspection.handleInspectionClick(partId)
  if (result.completed) {
    highlightedPartId = null
    stopTimer()
    playSuccess()
    const finalScore = Math.round(100 * settings.scoreMultiplier)
    recorder.savePracticeRecord('inspection', result.duration, finalScore, difficulty.value, {
      timeLimit: settings.timeLimit,
      correctParts: inspection.completedParts.value || []
    })
  } else if (result.nextHighlight) {
    highlightedPartId = result.nextHighlight
  } else if (result.wrongPart) {
    playFail()
  }
}

function handleInspectionUndo() {
  const prevPartId = inspection.undoInspectionStep()
  if (prevPartId) {
    highlightedPartId = prevPartId
    recorder.recordOperation(prevPartId, 'undo')
  }
}

function startDiagnosis() {
  diagnosis.resetDiagnosis()
  diagnosis.startDiagnosis()
  const settings = getDifficultySettings(difficulty.value)
  if (settings.timeLimit > 0) {
    timerLimit.value = settings.timeLimit
    timerActive.value = true
    timerRunning.value = true
  } else {
    stopTimer()
  }
}

function handleDiagnosisClick(partId) {
  const settings = getDifficultySettings(difficulty.value)
  const result = diagnosis.handleDiagnosisClick(partId)
  if (result.completed) {
    stopTimer()
    playSuccess()
    const finalScore = Math.round(result.score * settings.scoreMultiplier)
    recorder.savePracticeRecord('diagnosis', result.duration, finalScore, difficulty.value, {
      scenarioId: diagnosis.currentDiagnosis.value?.partId || null,
      timeLimit: settings.timeLimit,
      correctParts: [diagnosis.currentDiagnosis.value?.partId],
      wrongAttempts: diagnosis.diagnosisAttempts.value - 1
    })
  } else if (result.wrongPart) {
    playFail()
  }
}

function handleAssemblyClick(partId) {
  const partMeshes = getPartMeshes()
  const state = assemblyState.partStates[partId]

  selectPart(partId)

  if (state === 'attached') {
    if (canDetach(partId, assemblyState.partStates)) {
      detachPart(partId, partMeshes, assemblyState)
      playDetach()
      recorder.recordOperation(partId, 'detach')
    } else {
      const blockers = getDetachBlockers(partId, assemblyState.partStates)
      if (blockers.length > 0) {
        const blockerNames = blockers.map(id => getPartInfo(id)?.name || id).join('、')
        ElMessage.warning(`需要先拆卸：${blockerNames}`)
        playFail()
      }
    }
  } else if (state === 'detached') {
    if (canAttach(partId, assemblyState.partStates)) {
      attachPart(partId, partMeshes, assemblyState)
      playAttach()
      recorder.recordOperation(partId, 'attach')
    } else {
      const blockers = getAttachBlockers(partId, assemblyState.partStates)
      if (blockers.length > 0) {
        const blockerNames = blockers.map(id => getPartInfo(id)?.name || id).join('、')
        ElMessage.warning(`需要先装回：${blockerNames}`)
        playFail()
      }
    }
  }
}

function handleDetach(partId) {
  const partMeshes = getPartMeshes()
  if (canDetach(partId, assemblyState.partStates)) {
    detachPart(partId, partMeshes, assemblyState)
    playDetach()
    recorder.recordOperation(partId, 'detach')
  } else {
    const blockers = getDetachBlockers(partId, assemblyState.partStates)
    if (blockers.length > 0) {
      const blockerNames = blockers.map(id => getPartInfo(id)?.name || id).join('、')
      ElMessage.warning(`需要先拆卸：${blockerNames}`)
      playFail()
    }
  }
}

function handleAttach(partId) {
  const partMeshes = getPartMeshes()
  if (canAttach(partId, assemblyState.partStates)) {
    attachPart(partId, partMeshes, assemblyState)
    playAttach()
    recorder.recordOperation(partId, 'attach')
  } else {
    const blockers = getAttachBlockers(partId, assemblyState.partStates)
    if (blockers.length > 0) {
      const blockerNames = blockers.map(id => getPartInfo(id)?.name || id).join('、')
      ElMessage.warning(`需要先装回：${blockerNames}`)
      playFail()
    }
  }
}

function handleAssemblyReset() {
  const partMeshes = getPartMeshes()
  resetAssembly(partMeshes, assemblyState)
  ElMessage.success('已重置所有部件')
}

function handleShowReplay() {
  if (showReplay.value) {
    stopReplay()
    return
  }
  const ops = recorder.operations.value
  if (!ops || ops.length === 0) {
    ElMessage.info('暂无操作记录，请先进行实操练习')
    return
  }
  replayOperations.value = [...ops]
  replayStep.value = 0
  replayCurrentOp.value = null
  replayProgress.value = 0
  replayActive.value = false
  isReplayPlaying.value = false
  showReplay.value = true
}

function startReplay() {
  replayer.load(replayOperations.value)
  replayer.play()
  replayActive.value = true
  isReplayPlaying.value = true
  replayStep.value = 0
  if (sceneApi.controls) sceneApi.controls.enabled = false
}

function stopReplay() {
  replayer.stop()
  replayActive.value = false
  isReplayPlaying.value = false
  replayStep.value = 0
  replayCurrentOp.value = null
  replayProgress.value = 0
  showReplay.value = false
  const partMeshes = getPartMeshes()
  Object.values(partMeshes).forEach(mesh => unhighlightPart(mesh))
  if (sceneApi.controls) sceneApi.controls.enabled = true
  resetCamera()
}

function pauseReplay() {
  replayer.pause()
  isReplayPlaying.value = false
}

function resumeReplay() {
  replayer.play()
  isReplayPlaying.value = true
}

function seekReplay(index) {
  replayer.seek(index)
  replayStep.value = index
}

function setReplaySpeed(speed) {
  replaySpeed.value = speed
  replayer.setSpeed(speed)
}

function handleTimeout() {
  stopTimer()
  playFail()
  if (practiceMode.value === 'inspection') {
    inspection.practiceCompleted.value = true
    inspection.practiceDuration.value = getDifficultySettings(difficulty.value).timeLimit
    ElMessage.warning('时间到！检查未完成')
    recorder.savePracticeRecord('inspection', inspection.practiceDuration.value, 0, difficulty.value, {
      timeLimit: getDifficultySettings(difficulty.value).timeLimit
    })
  } else if (practiceMode.value === 'diagnosis') {
    diagnosis.practiceCompleted.value = true
    diagnosis.practiceDuration.value = getDifficultySettings(difficulty.value).timeLimit
    ElMessage.warning('时间到！诊断未完成')
    recorder.savePracticeRecord('diagnosis', diagnosis.practiceDuration.value, 0, difficulty.value, {
      timeLimit: getDifficultySettings(difficulty.value).timeLimit,
      wrongAttempts: diagnosis.diagnosisAttempts.value
    })
  }
}

function stopTimer() {
  timerRunning.value = false
  timerActive.value = false
}

function restartPractice() {
  clearSelection()
  stopTimer()
  const partMeshes = getPartMeshes()
  Object.values(partMeshes).forEach(mesh => unhighlightPart(mesh))
  if (practiceMode.value === 'inspection') startInspection()
  else if (practiceMode.value === 'diagnosis') startDiagnosis()
}

function resetCamera() {
  sceneApi.resetCamera()
}

function toggleFullscreen() {
  if (!practiceContainer.value) return
  if (!document.fullscreenElement) {
    practiceContainer.value.requestFullscreen().catch(() => ElMessage.warning('无法进入全屏模式'))
  } else {
    document.exitFullscreen()
  }
}

function takeScreenshot() {
  const renderer = getRenderer()
  if (!renderer) return
  try {
    renderer.render(sceneApi.scene, sceneApi.camera)
    const dataUrl = renderer.domElement.toDataURL('image/png')
    const link = document.createElement('a')
    link.download = `forklift-practice-${Date.now()}.png`
    link.href = dataUrl
    link.click()
    ElMessage.success('截图已保存')
  } catch (e) {
    ElMessage.error('截图失败')
  }
}

function finishTour() {
  showTour.value = false
  localStorage.setItem('practice_tour_done', '1')
}

function cleanup() {
  const renderer = getRenderer()
  if (renderer) {
    renderer.domElement.removeEventListener('pointerdown', onCanvasPointerDown)
    renderer.domElement.removeEventListener('pointerup', onCanvasPointerUp)
    renderer.domElement.removeEventListener('mousemove', onCanvasMouseMove)
    renderer.domElement.removeEventListener('contextmenu', onCanvasContextMenu)
  }
  sceneApi.unregisterAnimationCallback(onModeAnimationCallback)
  sceneApi.cleanup()
}

onMounted(() => {
  webglSupported.value = checkWebGL()
  if (webglSupported.value) {
    nextTick(() => {
      initScene()
      if (!localStorage.getItem('practice_tour_done')) {
        showTour.value = true
      }
    })
  }
})

onBeforeUnmount(() => {
  cleanup()
})
</script>

<style scoped>
.practice-page {
  height: calc(100vh - 160px);
  min-height: 500px;
}

.practice-container {
  position: relative;
  width: 100%;
  height: 100%;
  border-radius: 12px;
  overflow: hidden;
  background: #374151;
  -webkit-user-select: none;
  -moz-user-select: none;
  -ms-user-select: none;
  user-select: none;
}

.canvas-container {
  width: 100%;
  height: 100%;
  -webkit-user-select: none;
  -moz-user-select: none;
  -ms-user-select: none;
  user-select: none;
}

@media screen and (max-width: 768px) {
  .practice-page {
    height: calc(100vh - 84px);
    min-height: 400px;
  }
}
</style>
