import * as THREE from 'three'
import { highlightPart, unhighlightPart, getPartInfo } from '@/utils/forkliftModel'

export function createOperationReplayer() {
  let playing = false
  let paused = false
  let operations = []
  let currentIndex = 0
  let playbackSpeed = 1.0
  let elapsedTime = 0
  let onStepCallback = null
  let onCompleteCallback = null
  let onProgressCallback = null

  function load(ops) {
    operations = ops || []
    currentIndex = 0
    elapsedTime = 0
    playing = false
    paused = false
  }

  function play() {
    if (operations.length === 0) return
    playing = true
    paused = false
  }

  function pause() {
    paused = true
  }

  function stop() {
    playing = false
    paused = false
    currentIndex = 0
    elapsedTime = 0
  }

  function seek(index) {
    if (index < 0 || index >= operations.length) return
    currentIndex = index
    elapsedTime = 0
  }

  function setSpeed(speed) {
    playbackSpeed = Math.max(0.25, Math.min(5.0, speed))
  }

  function onStep(cb) {
    onStepCallback = cb
  }

  function onComplete(cb) {
    onCompleteCallback = cb
  }

  function onProgress(cb) {
    onProgressCallback = cb
  }

  function update(delta, partMeshes, camera, controls) {
    if (!playing || paused || operations.length === 0) return false

    elapsedTime += delta * playbackSpeed

    const stepDuration = 1.5 / playbackSpeed

    if (elapsedTime >= stepDuration) {
      elapsedTime = 0

      if (currentIndex > 0) {
        const prevOp = operations[currentIndex - 1]
        if (prevOp && partMeshes[prevOp.partId]) {
          unhighlightPart(partMeshes[prevOp.partId])
        }
      }

      if (currentIndex < operations.length) {
        const op = operations[currentIndex]
        if (partMeshes[op.partId]) {
          highlightPart(partMeshes[op.partId], '#00aaff', 0.5)
        }

        const partInfo = getPartInfo(op.partId)
        if (onStepCallback) {
          onStepCallback({
            index: currentIndex,
            operation: op,
            partInfo: partInfo,
            total: operations.length
          })
        }

        if (camera && controls && partInfo) {
          const labelOffset = partInfo.labelOffset || [0, 0, 0]
          const targetPos = new THREE.Vector3(labelOffset[0], labelOffset[1], labelOffset[2])
          controls.target.lerp(targetPos, 0.3)
          controls.update()
        }

        currentIndex++

        if (onProgressCallback) {
          onProgressCallback(currentIndex / operations.length)
        }

        if (currentIndex >= operations.length) {
          playing = false
          if (onCompleteCallback) onCompleteCallback()
          return true
        }
      }
    }

    return false
  }

  function getState() {
    return {
      playing: playing && !paused,
      paused,
      currentIndex,
      totalSteps: operations.length,
      progress: operations.length > 0 ? currentIndex / operations.length : 0,
      speed: playbackSpeed
    }
  }

  function isPlaying() {
    return playing && !paused
  }

  return {
    load,
    play,
    pause,
    stop,
    seek,
    setSpeed,
    onStep,
    onComplete,
    onProgress,
    update,
    getState,
    isPlaying
  }
}
