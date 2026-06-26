import * as THREE from 'three'

const DEMO_ANIMATIONS = {
  hydraulic_check: {
    name: '液压系统检查',
    description: '标准液压系统日常检查流程演示',
    duration: 12,
    keyframes: [
      { time: 0, camera: { pos: [4, 3, 4], target: [0, 0.8, 0] }, highlight: null, narration: '开始液压系统检查' },
      { time: 2, camera: { pos: [1, 1.5, 2], target: [0, 1.0, 0.65] }, highlight: 'hydraulic', narration: '检查液压缸外观，查看有无渗漏' },
      { time: 5, camera: { pos: [0.5, 0.8, 1.5], target: [0, 0.5, 0.65] }, highlight: 'hydraulic', narration: '检查液压油管路连接是否紧固' },
      { time: 8, camera: { pos: [2, 2, 3], target: [0, 0.8, 0] }, highlight: 'mast', narration: '操作升降杆，测试升降是否平稳' },
      { time: 11, camera: { pos: [4, 3, 4], target: [0, 0.8, 0] }, highlight: null, narration: '液压系统检查完成' }
    ]
  },
  fork_inspection: {
    name: '货叉检查',
    description: '货叉日常检查标准操作流程',
    duration: 10,
    keyframes: [
      { time: 0, camera: { pos: [4, 3, 4], target: [0, 0.8, 0] }, highlight: null, narration: '开始货叉检查' },
      { time: 2, camera: { pos: [0, 0.8, 2.5], target: [0, 0.2, 1.2] }, highlight: 'forks', narration: '检查货叉有无弯曲变形' },
      { time: 5, camera: { pos: [-0.5, 0.5, 2], target: [0.2, 0.15, 1.0] }, highlight: 'forks', narration: '查看叉尖磨损情况' },
      { time: 8, camera: { pos: [0, 1.5, 2], target: [0, 0.4, 0.72] }, highlight: 'forks', narration: '确认货叉定位销和挂架连接' },
      { time: 9.5, camera: { pos: [4, 3, 4], target: [0, 0.8, 0] }, highlight: null, narration: '货叉检查完成' }
    ]
  },
  daily_walkaround: {
    name: '日常巡检',
    description: '叉车日常巡检完整流程',
    duration: 18,
    keyframes: [
      { time: 0, camera: { pos: [4, 3, 4], target: [0, 0.8, 0] }, highlight: null, narration: '开始日常巡检' },
      { time: 2, camera: { pos: [0, 0.5, 2.5], target: [0, 0.2, 1.2] }, highlight: 'forks', narration: '第一步：检查货叉' },
      { time: 5, camera: { pos: [0, 2, 2], target: [0, 1.2, 0.75] }, highlight: 'mast', narration: '第二步：检查升降架' },
      { time: 8, camera: { pos: [1, 1.5, 1.5], target: [0, 1.0, 0.65] }, highlight: 'hydraulic', narration: '第三步：检查液压系统' },
      { time: 11, camera: { pos: [2, 0.5, 0], target: [0.58, 0.2, 0] }, highlight: 'wheels', narration: '第四步：检查车轮' },
      { time: 14, camera: { pos: [0, 2, -0.5], target: [0, 1.2, -0.2] }, highlight: 'cabin', narration: '第五步：检查驾驶室' },
      { time: 17, camera: { pos: [4, 3, 4], target: [0, 0.8, 0] }, highlight: null, narration: '日常巡检完成' }
    ]
  }
}

export function getDemoAnimations() {
  return DEMO_ANIMATIONS
}

export function getDemoAnimationList() {
  return Object.entries(DEMO_ANIMATIONS).map(([id, anim]) => ({
    id,
    name: anim.name,
    description: anim.description,
    duration: anim.duration
  }))
}

export function createAnimationPlayer() {
  let playing = false
  let paused = false
  let currentTime = 0
  let currentAnimation = null
  let playbackSpeed = 1.0
  let onCompleteCallback = null
  let onNarrationCallback = null
  let lastNarrationIndex = -1

  function load(animationId) {
    currentAnimation = DEMO_ANIMATIONS[animationId] || null
    currentTime = 0
    playing = false
    paused = false
    lastNarrationIndex = -1
  }

  function play() {
    if (!currentAnimation) return
    playing = true
    paused = false
  }

  function pause() {
    paused = true
  }

  function resume() {
    paused = false
  }

  function stop() {
    playing = false
    paused = false
    currentTime = 0
    lastNarrationIndex = -1
  }

  function seek(time) {
    if (!currentAnimation) return
    currentTime = Math.max(0, Math.min(time, currentAnimation.duration))
    lastNarrationIndex = -1
  }

  function setSpeed(speed) {
    playbackSpeed = Math.max(0.25, Math.min(3.0, speed))
  }

  function onComplete(cb) {
    onCompleteCallback = cb
  }

  function onNarration(cb) {
    onNarrationCallback = cb
  }

  function update(delta, camera, controls, partMeshes, highlightFn, unhighlightAllFn) {
    if (!playing || paused || !currentAnimation) return false

    currentTime += delta * playbackSpeed

    if (currentTime >= currentAnimation.duration) {
      currentTime = currentAnimation.duration
      playing = false
      if (onCompleteCallback) onCompleteCallback()
      return true
    }

    const keyframes = currentAnimation.keyframes
    let prevKf = keyframes[0]
    let nextKf = keyframes[keyframes.length - 1]

    for (let i = 0; i < keyframes.length - 1; i++) {
      if (currentTime >= keyframes[i].time && currentTime < keyframes[i + 1].time) {
        prevKf = keyframes[i]
        nextKf = keyframes[i + 1]
        break
      }
    }

    const segmentDuration = nextKf.time - prevKf.time
    const t = segmentDuration > 0 ? (currentTime - prevKf.time) / segmentDuration : 0
    const smoothT = t * t * (3 - 2 * t)

    if (camera && prevKf.camera && nextKf.camera) {
      const pos = new THREE.Vector3().lerpVectors(
        new THREE.Vector3(...prevKf.camera.pos),
        new THREE.Vector3(...nextKf.camera.pos),
        smoothT
      )
      const target = new THREE.Vector3().lerpVectors(
        new THREE.Vector3(...prevKf.camera.target),
        new THREE.Vector3(...nextKf.camera.target),
        smoothT
      )
      camera.position.copy(pos)
      if (controls) {
        controls.target.copy(target)
        controls.update()
      }
    }

    if (unhighlightAllFn) unhighlightAllFn()

    const currentKf = prevKf
    if (currentKf.highlight && partMeshes[currentKf.highlight] && highlightFn) {
      highlightFn(partMeshes[currentKf.highlight], '#00ff88', 0.4)
    }

    const narrationIndex = keyframes.findIndex(kf => kf.time > currentTime) - 1
    if (narrationIndex >= 0 && narrationIndex !== lastNarrationIndex) {
      lastNarrationIndex = narrationIndex
      if (onNarrationCallback && keyframes[narrationIndex].narration) {
        onNarrationCallback(keyframes[narrationIndex].narration)
      }
    }

    return false
  }

  function getState() {
    return {
      playing,
      paused,
      currentTime,
      duration: currentAnimation ? currentAnimation.duration : 0,
      progress: currentAnimation ? currentTime / currentAnimation.duration : 0,
      animationName: currentAnimation ? currentAnimation.name : '',
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
    resume,
    stop,
    seek,
    setSpeed,
    onComplete,
    onNarration,
    update,
    getState,
    isPlaying
  }
}
