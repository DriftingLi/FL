import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { Ref } from 'vue'
import { credentialApi, type CredentialDict, type GroupedCredentials } from '@/api/credential'

export const useCredentialStore = defineStore('credential', () => {
  const current: Ref<CredentialDict | null> = ref(null)
  const grouped: Ref<GroupedCredentials> = ref({ special_operation: [], skill_level: [] })
  const flatList: Ref<CredentialDict[]> = ref([])
  const loading: Ref<boolean> = ref(false)
  const initialized: Ref<boolean> = ref(false)

  async function loadGrouped() {
    loading.value = true
    try {
      const data = await credentialApi.listGrouped()
      grouped.value = data
      flatList.value = [...(data.special_operation || []), ...(data.skill_level || [])]
    } finally {
      loading.value = false
    }
  }

  async function loadFlat() {
    const data = await credentialApi.listCredentials()
    flatList.value = data.credentials || []
    // also populate grouped from flat if needed
    grouped.value = {
      special_operation: flatList.value.filter(c => c.category === 'special_operation'),
      skill_level: flatList.value.filter(c => c.category === 'skill_level')
    }
  }

  async function loadCurrent(): Promise<CredentialDict | null> {
    try {
      const data = await credentialApi.getCurrent()
      current.value = data.credential || null
      initialized.value = true
      return current.value
    } catch {
      current.value = null
      initialized.value = true
      return null
    }
  }

  async function ensureInitialized(): Promise<void> {
    if (initialized.value) return
    await loadCurrent()
  }

  async function switchTo(credentialId: number): Promise<CredentialDict> {
    const data = await credentialApi.setCurrent(credentialId)
    current.value = data.credential
    // 同步到 auth 的 localStorage userInfo（如有）
    try {
      const raw = localStorage.getItem('userInfo')
      if (raw) {
        const info = JSON.parse(raw)
        info.current_credential_id = credentialId
        info.current_credential = data.credential
        localStorage.setItem('userInfo', JSON.stringify(info))
      }
    } catch {}
    return data.credential
  }

  async function initialize(): Promise<void> {
    await Promise.all([loadGrouped().catch(() => {}), loadCurrent().catch(() => {})])
  }

  return {
    current,
    grouped,
    flatList,
    loading,
    initialized,
    loadGrouped,
    loadFlat,
    loadCurrent,
    ensureInitialized,
    switchTo,
    initialize
  }
})
