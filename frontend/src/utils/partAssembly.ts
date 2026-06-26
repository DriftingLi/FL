import * as THREE from 'three'

const ASSEMBLY_DEPENDENCIES = {
  overheadGuard: { requiresDetached: [], blocks: ['cabin'] },
  cabin: { requiresDetached: ['overheadGuard'], blocks: [] },
  forks: { requiresDetached: [], blocks: [] },
  mast: { requiresDetached: ['forks'], blocks: ['hydraulic'] },
  hydraulic: { requiresDetached: ['mast'], blocks: [] },
  wheels: { requiresDetached: [], blocks: [] },
  counterweight: { requiresDetached: [], blocks: [] },
  body: { requiresDetached: ['cabin', 'wheels', 'counterweight', 'mast'], blocks: [] }
}

const DETACH_OFFSETS = {
  overheadGuard: new THREE.Vector3(0, 1.0, 0),
  cabin: new THREE.Vector3(0, 0.8, 0),
  forks: new THREE.Vector3(0, 0, 1.5),
  mast: new THREE.Vector3(0, 0.5, 1.2),
  hydraulic: new THREE.Vector3(0.5, 0.5, 0.8),
  wheels: new THREE.Vector3(1.2, 0, 0),
  counterweight: new THREE.Vector3(0, 0, -1.2),
  body: new THREE.Vector3(0, 0.5, 0)
}

const SNAP_DISTANCE = 0.5
const LERP_SPEED = 0.08

export function createAssemblyState() {
  const partStates = {}
  const partOriginals = {}
  const partTargets = {}

  Object.keys(ASSEMBLY_DEPENDENCIES).forEach(partId => {
    partStates[partId] = 'attached'
  })

  return { partStates, partOriginals, partTargets }
}

export function canDetach(partId, partStates) {
  const deps = ASSEMBLY_DEPENDENCIES[partId]
  if (!deps) return false

  if (partStates[partId] !== 'attached') return false

  for (const blockerId of deps.blocks) {
    if (partStates[blockerId] === 'attached') {
      return false
    }
  }

  return true
}

export function canAttach(partId, partStates) {
  if (!partStates[partId] || partStates[partId] !== 'detached') return false

  const deps = ASSEMBLY_DEPENDENCIES[partId]
  if (!deps) return false

  for (const reqId of deps.requiresDetached) {
    if (partStates[reqId] === 'detached') {
      return false
    }
  }

  return true
}

export function getDetachBlockers(partId, partStates) {
  const deps = ASSEMBLY_DEPENDENCIES[partId]
  if (!deps) return []
  return deps.blocks.filter(id => partStates[id] === 'attached')
}

export function getAttachBlockers(partId, partStates) {
  const deps = ASSEMBLY_DEPENDENCIES[partId]
  if (!deps) return []
  return deps.requiresDetached.filter(id => partStates[id] === 'detached')
}

export function detachPart(partId, partMeshes, state) {
  if (!canDetach(partId, state.partStates)) return false

  const mesh = partMeshes[partId]
  if (!mesh) return false

  state.partOriginals[partId] = {
    position: mesh.position.clone(),
    rotation: mesh.rotation.clone()
  }

  const offset = DETACH_OFFSETS[partId] || new THREE.Vector3(0, 0.5, 0)
  const targetPos = mesh.position.clone().add(offset)
  state.partTargets[partId] = targetPos

  state.partStates[partId] = 'detaching'
  return true
}

export function attachPart(partId, partMeshes, state) {
  if (!canAttach(partId, state.partStates)) return false

  const mesh = partMeshes[partId]
  if (!mesh || !state.partOriginals[partId]) return false

  state.partTargets[partId] = state.partOriginals[partId].position.clone()
  state.partStates[partId] = 'attaching'
  return true
}

export function updateAssemblyAnimation(partMeshes, state) {
  let animating = false

  Object.keys(state.partStates).forEach(partId => {
    const meshState = state.partStates[partId]
    if (meshState !== 'detaching' && meshState !== 'attaching') return

    const mesh = partMeshes[partId]
    const target = state.partTargets[partId]
    if (!mesh || !target) return

    mesh.position.lerp(target, LERP_SPEED)

    if (mesh.position.distanceTo(target) < 0.01) {
      mesh.position.copy(target)
      if (meshState === 'attaching') {
        state.partStates[partId] = 'attached'
        delete state.partTargets[partId]
      } else {
        state.partStates[partId] = 'detached'
      }
    }

    animating = true
  })

  return animating
}

export function isNearOriginal(partId, partMeshes, state) {
  const mesh = partMeshes[partId]
  const original = state.partOriginals[partId]
  if (!mesh || !original) return false
  return mesh.position.distanceTo(original.position) < SNAP_DISTANCE
}

export function getAssemblyOrder() {
  return [
    'overheadGuard',
    'forks',
    'wheels',
    'counterweight',
    'mast',
    'hydraulic',
    'cabin',
    'body'
  ]
}

export function getDisassemblyOrder() {
  return [
    'overheadGuard',
    'cabin',
    'forks',
    'mast',
    'hydraulic',
    'wheels',
    'counterweight',
    'body'
  ]
}

export function resetAssembly(partMeshes, state) {
  Object.keys(state.partStates).forEach(partId => {
    const mesh = partMeshes[partId]
    const original = state.partOriginals[partId]
    if (mesh && original) {
      mesh.position.copy(original.position)
      mesh.rotation.copy(original.rotation)
    }
    state.partStates[partId] = 'attached'
  })
  state.partOriginals = {}
  state.partTargets = {}
}

export function getDetachedParts(state) {
  return Object.keys(state.partStates).filter(id => state.partStates[id] === 'detached')
}

export function getAttachedParts(state) {
  return Object.keys(state.partStates).filter(id => state.partStates[id] === 'attached')
}
