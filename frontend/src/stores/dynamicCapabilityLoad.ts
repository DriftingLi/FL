import { computed, ref } from 'vue'
import { isDynamicRole } from '@/utils/authzRuntime'

export type DynamicCapabilityLoadState = 'idle' | 'loading' | 'ready' | 'failed'

export interface DynamicCapabilityIdentity {
  key: string
  role: string
}

interface CapabilityResponse {
  capabilities?: readonly string[] | null
}

export interface DynamicCapabilityAdapters {
  admin: () => Promise<CapabilityResponse | null | undefined>
  tutor: () => Promise<CapabilityResponse | null | undefined>
}

/**
 * 动态能力装载的唯一 owner（spec #2920 切片 4）。
 *
 * 身份 key 变化即作废在途代次；端点选择、并发去重、失败落定与显式重试都收在 module 内。
 * auth store 只保存并暴露结果，router guard 只消费 settled 标志。
 */
export function createDynamicCapabilityLoad(adapters: DynamicCapabilityAdapters) {
  const state = ref<DynamicCapabilityLoadState>('idle')
  const capabilities = ref<string[]>([])
  const settled = computed(() => state.value === 'ready' || state.value === 'failed')

  let identityKey = ''
  let role = ''
  let generation = 0
  let inFlight: Promise<void> | null = null

  function setIdentity(identity: DynamicCapabilityIdentity): void {
    if (identity.key === identityKey && identity.role === role) return
    identityKey = identity.key
    role = identity.role
    generation += 1
    inFlight = null
    capabilities.value = []
    state.value = 'idle'
  }

  function load(): Promise<void> {
    const requestRole = role
    if (!identityKey || !isDynamicRole(requestRole)) {
      state.value = 'idle'
      return Promise.resolve()
    }
    if (inFlight) return inFlight

    const requestGeneration = generation
    state.value = 'loading'
    const promise = (async () => {
      try {
        const response = requestRole === 'tutor'
          ? await adapters.tutor()
          : await adapters.admin()
        if (requestGeneration !== generation) return
        capabilities.value = [...(response?.capabilities ?? [])]
        state.value = 'ready'
      } catch (error) {
        if (requestGeneration !== generation) return
        capabilities.value = []
        state.value = 'failed'
        console.warn('[Auth] loadCapabilities failed:', error)
      } finally {
        if (requestGeneration === generation) inFlight = null
      }
    })()
    inFlight = promise
    return promise
  }

  function reset(): void {
    setIdentity({ key: '', role: '' })
  }

  return { state, capabilities, settled, setIdentity, load, reset }
}
