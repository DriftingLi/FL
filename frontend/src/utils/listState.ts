import type { ApiErrorKind } from '@/api/client'

/**
 * 列表三态里「空条目的值」这一支的**唯一判据实现**（第十一波 #1101）。
 *
 * 抽出来的原因：ADR-0053 §6 落地时判据长在 `useAsyncPage` 里（`isEmpty`），但有两个
 * 消费面拿不到那一个 ref——① 一次装载按 Tab 二选一写两个列表的页面（ForumPage）；
 * ② 判据形状特殊的页面（以 total 为准等）。它们此前只能把 `X.length === 0` 再抄一遍，
 * 于是同一条不变式又散成多份。本 module 让这些页面复用同一份判定而不是复制。
 *
 * 判定口径（与 ADR-0056 §7 一致）：
 * - `loading` 中恒 false（空态在四段式里的合法位置只在装载完成之后）；
 * - 「没有条目」（null / 空数组）= 空态；
 * - **404（资源不存在）= 空态**，其余错误 = 错误态 —— 复用 `api/client.ts` 的
 *   `ApiErrorKind`（拦截器挂在错误对象上的既有分类），不另造一份错误分类。
 */
export type EmptyableValue = { length: number } | object | undefined | null

/** 值本身是不是「没有条目」：null / undefined / 空数组。单对象非空即视为有条目。 */
export function isEmptyValue(value: EmptyableValue): boolean {
  if (value === null || value === undefined) return true
  if (Array.isArray(value)) return value.length === 0
  return false
}

/** 空态判据：值为空且（未出错或错在 404）。 */
export function isEmptyList(
  value: EmptyableValue,
  state: { error: boolean; kind: ApiErrorKind | null }
): boolean {
  if (!isEmptyValue(value)) return false
  return !state.error || state.kind === 'notfound'
}
