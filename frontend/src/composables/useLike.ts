export interface ForumLikeResult {
  likes_count: number
  liked: boolean
}

export interface ForumLikeTarget {
  id: number
  liked_by_me?: boolean
  likes_count?: number
}

/**
 * 论坛点赞（#389 单点）：乐观更新 + 失败回滚。
 *
 * toggle 先按 ±1 乐观改写目标对象（liked_by_me / likes_count），
 * 成功后以服务端返回计数收敛，失败回滚到先前置 —— 帖子与回复共用同一份时序。
 * adapters 由调用方注入（likeTopic/unlikeTopic 或 likeReply/unlikeReply）。
 */
export function useLike(
  like: (id: number) => Promise<ForumLikeResult>,
  unlike: (id: number) => Promise<ForumLikeResult>
) {
  async function toggle(target: ForumLikeTarget): Promise<void> {
    const prevLiked = !!target.liked_by_me
    const prevCount = target.likes_count || 0
    // 乐观更新
    target.liked_by_me = !prevLiked
    target.likes_count = prevLiked ? Math.max(0, prevCount - 1) : prevCount + 1
    try {
      const res = prevLiked ? await unlike(target.id) : await like(target.id)
      target.likes_count = res.likes_count
      target.liked_by_me = res.liked
    } catch (e) {
      // 回滚
      target.liked_by_me = prevLiked
      target.likes_count = prevCount
      console.error('点赞操作失败:', e)
    }
  }

  return { toggle }
}
