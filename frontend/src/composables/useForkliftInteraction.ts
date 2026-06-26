import { ref } from 'vue'
import { getPartInfo, highlightPart, unhighlightPart } from '@/utils/forkliftModel'

export function useForkliftInteraction(getPartMeshes, getRenderer) {
  const selectedPart = ref(null)
  const hoveredPartId = ref(null)
  const highlightedPartId = ref(null)

  function selectPart(partId) {
    const partMeshes = getPartMeshes()
    if (highlightedPartId.value && highlightedPartId.value !== partId && partMeshes[highlightedPartId.value]) {
      unhighlightPart(partMeshes[highlightedPartId.value])
    }
    highlightedPartId.value = partId
    if (partMeshes[partId]) {
      highlightPart(partMeshes[partId], '#00aaff', 0.3)
    }
    selectedPart.value = getPartInfo(partId)
  }

  function clearSelection() {
    const partMeshes = getPartMeshes()
    if (highlightedPartId.value && partMeshes[highlightedPartId.value]) {
      unhighlightPart(partMeshes[highlightedPartId.value])
    }
    highlightedPartId.value = null
    selectedPart.value = null
  }

  function setHighlightedPart(partId, color = '#00ff88', intensity = 0.3) {
    const partMeshes = getPartMeshes()
    if (highlightedPartId.value && partMeshes[highlightedPartId.value] && highlightedPartId.value !== partId) {
      unhighlightPart(partMeshes[highlightedPartId.value])
    }
    highlightedPartId.value = partId
    if (partMeshes[partId]) {
      highlightPart(partMeshes[partId], color, intensity)
    }
  }

  function unhighlightAll() {
    const partMeshes = getPartMeshes()
    Object.values(partMeshes).forEach(mesh => unhighlightPart(mesh))
    highlightedPartId.value = null
  }

  function onCanvasClick(event, practiceMode, handlers) {
    const renderer = getRenderer()
    if (!renderer) return

    const rect = renderer.domElement.getBoundingClientRect()
    const intersects = getIntersectedParts(event.clientX, event.clientY, rect)

    if (intersects.length > 0) {
      const partId = findPartId(intersects[0])
      if (partId) {
        const partInfo = getPartInfo(partId)
        if (!partInfo) return

        if (practiceMode === 'free') {
          selectPart(partId)
        } else if (practiceMode === 'inspection' && handlers.onInspectionClick) {
          handlers.onInspectionClick(partId)
        } else if (practiceMode === 'diagnosis' && handlers.onDiagnosisClick) {
          handlers.onDiagnosisClick(partId)
        }
      }
    } else {
      if (practiceMode === 'free') {
        clearSelection()
      }
    }
  }

  function onCanvasMouseMove(event) {
    const renderer = getRenderer()
    if (!renderer) return

    const rect = renderer.domElement.getBoundingClientRect()
    const intersects = getIntersectedParts(event.clientX, event.clientY, rect)

    if (intersects.length > 0) {
      const partId = findPartId(intersects[0])
      if (partId && partId !== hoveredPartId.value) {
        hoveredPartId.value = partId
        renderer.domElement.style.cursor = 'pointer'
      } else if (!partId) {
        hoveredPartId.value = null
        renderer.domElement.style.cursor = 'default'
      }
    } else {
      hoveredPartId.value = null
      renderer.domElement.style.cursor = 'default'
    }
  }

  function onCanvasMouseDown(event) {
    if (event.button === 2) {
      event.preventDefault()
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

  function getIntersectedParts(clientX, clientY, rect) {
    return []
  }

  function findPartId(hitObject) {
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

  return {
    selectedPart,
    hoveredPartId,
    highlightedPartId,
    selectPart,
    clearSelection,
    setHighlightedPart,
    unhighlightAll,
    onCanvasClick,
    onCanvasMouseMove,
    onCanvasMouseDown,
    onCanvasContextMenu,
    onContainerContextMenu
  }
}
