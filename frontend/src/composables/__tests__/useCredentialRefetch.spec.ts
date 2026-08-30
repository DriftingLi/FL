// useCredentialRefetch：证件切换即重拉单点（#387）的接口级测试。
// seam：composable 接口——以真实 pinia store 为信号源，watch 触发即计数。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { nextTick } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import { useCredentialStore } from '@/stores/credential'
import { useCredentialRefetch } from '../useCredentialRefetch'
import type { CredentialDict } from '@/api/credential'

function credentialOf(id: number): CredentialDict {
  return { id, code: `C${id}`, name: `证件${id}`, description: '', category: 'special_operation', level: null, sort_order: 0, status: 1, created_at: '', updated_at: '' }
}

beforeEach(() => {
  setActivePinia(createPinia())
})

describe('useCredentialRefetch（证件切换即重拉）', () => {
  it('当前证件 id 变化时触发 refetch', async () => {
    const store = useCredentialStore()
    const refetch = vi.fn()
    useCredentialRefetch(refetch)

    store.current = credentialOf(1)
    await nextTick()
    expect(refetch).toHaveBeenCalledTimes(1)

    store.current = credentialOf(2)
    await nextTick()
    expect(refetch).toHaveBeenCalledTimes(2)
  })

  it('id 不变（重设同名证件对象）不重复触发', async () => {
    const store = useCredentialStore()
    store.current = credentialOf(1)
    const refetch = vi.fn()
    useCredentialRefetch(refetch)

    store.current = credentialOf(1)
    await nextTick()
    expect(refetch).not.toHaveBeenCalled()
  })

  it('清空当前证件（null）同样视为变化并触发', async () => {
    const store = useCredentialStore()
    store.current = credentialOf(3)
    const refetch = vi.fn()
    useCredentialRefetch(refetch)

    store.current = null
    await nextTick()
    expect(refetch).toHaveBeenCalledTimes(1)
  })
})
