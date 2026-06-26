import { ref } from 'vue'
import * as THREE from 'three'
import { OrbitControls } from 'three/examples/jsm/controls/OrbitControls.js'
import { CSS2DRenderer } from 'three/examples/jsm/renderers/CSS2DRenderer.js'
import { createForklift, createWorkshop, clearGeometryCache, createPartLabels } from '@/utils/forkliftModel'

export function useThreeScene(canvasContainer, lowQualityMode) {
  let scene = null
  let camera = null
  let renderer = null
  let controls = null
  let forkliftGroup = null
  let partMeshes = {}
  let clock = null
  let animationId = null
  let raycaster = null
  let mouse = null
  let labelRenderer = null
  let partLabels = {}

  const isSceneReady = ref(false)
  const currentFps = ref(60)

  const animationCallbacks = []

  let fpsFrames = 0
  let fpsLastTime = 0
  let lowFpsCount = 0
  let autoDowngraded = false

  let needsRender = true
  let lastInteractionTime = 0
  const IDLE_THRESHOLD = 3000

  function registerAnimationCallback(cb) {
    animationCallbacks.push(cb)
    needsRender = true
  }

  function unregisterAnimationCallback(cb) {
    const idx = animationCallbacks.indexOf(cb)
    if (idx > -1) animationCallbacks.splice(idx, 1)
  }

  function hasActiveAnimations() {
    return animationCallbacks.length > 0
  }

  function markInteraction() {
    lastInteractionTime = performance.now()
    needsRender = true
  }

  function initScene() {
    const container = canvasContainer.value
    if (!container) return

    const width = container.clientWidth
    const height = container.clientHeight

    scene = new THREE.Scene()
    scene.background = new THREE.Color('#374151')
    scene.fog = new THREE.Fog('#374151', 15, 30)

    camera = new THREE.PerspectiveCamera(55, width / height, 0.1, 100)
    camera.position.set(4, 3, 4)

    renderer = new THREE.WebGLRenderer({
      antialias: !lowQualityMode.value,
      alpha: false
    })
    renderer.setSize(width, height)
    renderer.setPixelRatio(lowQualityMode.value ? 1 : Math.min(window.devicePixelRatio, 2))
    renderer.shadowMap.enabled = !lowQualityMode.value
    renderer.shadowMap.type = THREE.PCFSoftShadowMap
    renderer.toneMapping = THREE.ACESFilmicToneMapping
    renderer.toneMappingExposure = 1.2
    container.appendChild(renderer.domElement)

    controls = new OrbitControls(camera, renderer.domElement)
    controls.enableDamping = true
    controls.dampingFactor = 0.08
    controls.minDistance = 2
    controls.maxDistance = 12
    controls.maxPolarAngle = Math.PI / 2 + 0.1
    controls.target.set(0, 0.8, 0)
    controls.mouseButtons = {
      LEFT: THREE.MOUSE.ROTATE,
      MIDDLE: THREE.MOUSE.DOLLY,
      RIGHT: THREE.MOUSE.PAN
    }
    controls.update()

    controls.addEventListener('start', markInteraction)
    controls.addEventListener('change', markInteraction)

    const ambientLight = new THREE.AmbientLight('#ffffff', 0.5)
    scene.add(ambientLight)

    const dirLight = new THREE.DirectionalLight('#ffffff', 1.0)
    dirLight.position.set(5, 8, 5)
    dirLight.castShadow = true
    dirLight.shadow.mapSize.width = lowQualityMode.value ? 512 : 2048
    dirLight.shadow.mapSize.height = lowQualityMode.value ? 512 : 2048
    dirLight.shadow.camera.near = 0.5
    dirLight.shadow.camera.far = 25
    dirLight.shadow.camera.left = -6
    dirLight.shadow.camera.right = 6
    dirLight.shadow.camera.top = 6
    dirLight.shadow.camera.bottom = -6
    scene.add(dirLight)

    const fillLight = new THREE.DirectionalLight('#90cdf4', 0.3)
    fillLight.position.set(-3, 4, -3)
    scene.add(fillLight)

    const workshop = createWorkshop()
    scene.add(workshop)

    const { group, partMeshes: meshes } = createForklift()
    forkliftGroup = group
    partMeshes = meshes
    scene.add(forkliftGroup)

    partLabels = createPartLabels()
    Object.values(partLabels).forEach(label => {
      forkliftGroup.add(label)
    })

    labelRenderer = new CSS2DRenderer()
    labelRenderer.setSize(width, height)
    labelRenderer.domElement.style.position = 'absolute'
    labelRenderer.domElement.style.top = '0'
    labelRenderer.domElement.style.left = '0'
    labelRenderer.domElement.style.pointerEvents = 'none'
    container.appendChild(labelRenderer.domElement)

    raycaster = new THREE.Raycaster()
    mouse = new THREE.Vector2()
    clock = new THREE.Clock()

    window.addEventListener('resize', onWindowResize)
    isSceneReady.value = true
    lastInteractionTime = performance.now()
    fpsLastTime = performance.now()
    animate()
  }

  function animate() {
    animationId = requestAnimationFrame(animate)

    const now = performance.now()
    fpsFrames++
    if (now - fpsLastTime >= 1000) {
      currentFps.value = fpsFrames
      if (fpsFrames < 25 && !autoDowngraded && !lowQualityMode.value) {
        lowFpsCount++
        if (lowFpsCount >= 3) {
          autoDowngrade()
        }
      } else if (fpsFrames >= 30) {
        lowFpsCount = Math.max(0, lowFpsCount - 1)
      }
      fpsFrames = 0
      fpsLastTime = now
    }

    const hasAnimations = hasActiveAnimations()
    const isInteracting = (now - lastInteractionTime) < IDLE_THRESHOLD

    if (!hasAnimations && !isInteracting) {
      if (needsRender) {
        needsRender = false
      } else {
        return
      }
    } else {
      needsRender = true
    }

    const delta = Math.min(clock.getDelta(), 0.1)
    const elapsed = clock.elapsedTime

    for (const cb of animationCallbacks) {
      cb(elapsed, delta)
    }

    controls.update()
    renderer.render(scene, camera)
    if (labelRenderer) {
      labelRenderer.render(scene, camera)
    }
  }

  function autoDowngrade() {
    autoDowngraded = true
    lowQualityMode.value = true
    if (renderer) {
      renderer.setPixelRatio(1)
      renderer.shadowMap.enabled = false
      renderer.shadowMap.needsUpdate = true
    }
  }

  function onWindowResize() {
    if (!camera || !renderer || !canvasContainer.value) return
    const width = canvasContainer.value.clientWidth
    const height = canvasContainer.value.clientHeight
    camera.aspect = width / height
    camera.updateProjectionMatrix()
    renderer.setSize(width, height)
    if (labelRenderer) labelRenderer.setSize(width, height)
    needsRender = true
  }

  function resetCamera() {
    if (!controls || !camera) return
    camera.position.set(4, 3, 4)
    controls.target.set(0, 0.8, 0)
    controls.update()
    markInteraction()
  }

  function handleQualityChange(low) {
    if (!renderer) return
    autoDowngraded = false
    lowFpsCount = 0
    if (low) {
      renderer.setPixelRatio(1)
      renderer.shadowMap.enabled = false
    } else {
      renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2))
      renderer.shadowMap.enabled = true
    }
    renderer.shadowMap.needsUpdate = true
    needsRender = true
  }

  function getRaycasterIntersection(clientX, clientY) {
    if (!renderer || !camera || !raycaster) return []
    const rect = renderer.domElement.getBoundingClientRect()
    mouse.x = ((clientX - rect.left) / rect.width) * 2 - 1
    mouse.y = -((clientY - rect.top) / rect.height) * 2 + 1
    raycaster.setFromCamera(mouse, camera)

    const clickableObjects = []
    if (forkliftGroup) {
      forkliftGroup.traverse(child => {
        if (child.isMesh) clickableObjects.push(child)
      })
    }
    return raycaster.intersectObjects(clickableObjects, false)
  }

  function findPartIdFromIntersection(intersects) {
    if (intersects.length === 0) return null
    const hitObject = intersects[0].object
    let partId = hitObject.userData.partId
    if (!partId && hitObject.parent) {
      partId = hitObject.parent.userData.partId
    }
    if (!partId) {
      let parent = hitObject.parent
      while (parent && !partId) {
        partId = parent.userData.partId
        parent = parent.parent
      }
    }
    return partId
  }

  function cleanup() {
    if (animationId) {
      cancelAnimationFrame(animationId)
      animationId = null
    }
    if (controls) {
      controls.removeEventListener('start', markInteraction)
      controls.removeEventListener('change', markInteraction)
      controls.dispose()
    }
    if (renderer) {
      renderer.dispose()
    }
    if (labelRenderer && labelRenderer.domElement && labelRenderer.domElement.parentNode) {
      labelRenderer.domElement.parentNode.removeChild(labelRenderer.domElement)
    }
    labelRenderer = null
    partLabels = {}
    if (scene) {
      scene.traverse(child => {
        if (child.isMesh) {
          if (child.material.isMaterial) {
            child.material.dispose()
          }
        }
      })
    }
    clearGeometryCache()
    window.removeEventListener('resize', onWindowResize)
    scene = null
    camera = null
    renderer = null
    controls = null
    forkliftGroup = null
    partMeshes = {}
    isSceneReady.value = false
  }

  return {
    get scene() { return scene },
    get camera() { return camera },
    get renderer() { return renderer },
    get controls() { return controls },
    get forkliftGroup() { return forkliftGroup },
    get partMeshes() { return partMeshes },
    get partLabels() { return partLabels },
    get clock() { return clock },
    isSceneReady,
    currentFps,
    initScene,
    cleanup,
    resetCamera,
    handleQualityChange,
    getRaycasterIntersection,
    findPartIdFromIntersection,
    registerAnimationCallback,
    unregisterAnimationCallback,
    markInteraction
  }
}
