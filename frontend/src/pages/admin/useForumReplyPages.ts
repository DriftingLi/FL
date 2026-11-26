import { reactive } from 'vue'
import { adminForumApi, type AdminForumReply } from '@/api/forum'
import { isEmptyList } from '@/utils/listState'

const PAGE_SIZE = 20

interface ReplyPages {
  replies: AdminForumReply[] | null
  page: number
  pages: number
  total: number
  loading: boolean
  loadingMore: boolean
  error: boolean
  moreError: boolean
}

export function useForumReplyPages() {
  const replyPages = reactive<Partial<Record<number, ReplyPages>>>({})

  function invalidateReplies(topicId: number) {
    delete replyPages[topicId]
  }

  async function loadPage(topicId: number, state: ReplyPages, append: boolean) {
    const page = append ? state.page + 1 : 1
    state.loading = !append
    state.loadingMore = append
    state.error = false
    state.moreError = false
    try {
      const result = await adminForumApi.getTopic(topicId, page, PAGE_SIZE)
      if (replyPages[topicId] !== state) return
      const items = result.replies ?? []
      state.replies = append ? [...(state.replies ?? []), ...items] : items
      state.page = result.page ?? page
      state.pages = result.pages ?? state.pages
      state.total = result.total ?? state.total
    } catch {
      if (replyPages[topicId] !== state) return
      state.error = !append
      state.moreError = append
    } finally {
      state.loading = false
      state.loadingMore = false
    }
  }

  async function loadReplies(topicId: number) {
    const state = (replyPages[topicId] ??= reactive({
      replies: null,
      page: 0,
      pages: 1,
      total: 0,
      loading: false,
      loadingMore: false,
      error: false,
      moreError: false
    }))
    if (state.replies !== null || state.loading) return
    await loadPage(topicId, state, false)
  }

  async function loadMoreReplies(topicId: number) {
    const state = replyPages[topicId]
    if (!state?.replies || state.loadingMore || state.page >= state.pages) return
    await loadPage(topicId, state, true)
  }

  function refreshReplies(topicId: number) {
    invalidateReplies(topicId)
    return loadReplies(topicId)
  }

  function isEmptyReplies(topicId: number) {
    const state = replyPages[topicId]
    return isEmptyList(state?.replies ?? null, { error: state?.error ?? false, kind: null })
  }

  return { replyPages, loadReplies, loadMoreReplies, refreshReplies, invalidateReplies, isEmptyReplies }
}
