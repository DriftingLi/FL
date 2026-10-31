import { computed, nextTick, ref } from 'vue'

/** 一组「从里面选一个」的选项。分段控件与下划线选项卡的选项形状相同。 */
export interface RovingOption {
  label: string
  value: string
}

export interface RovingSelectionOptions<T extends RovingOption> {
  /** 当前选项集合。传 getter：props 变化时要读到最新值，不能在 setup 时拍死。 */
  options: () => T[]
  /** 当前选中值 */
  current: () => string
  /** 整组是否禁用 */
  disabled: () => boolean
  /** 选中某一项（点击与方向键共用同一条写入口） */
  select: (option: T) => void
}

/**
 * 「单选」控件族的 roving tabindex + 方向键导航（#2156）。
 *
 * 分段控件（radiogroup）与下划线选项卡（tablist）的角色不同，但键盘手感必须同族：
 * 各抄一份 switch 之后会慢慢分叉（一处加了 Home/End、另一处没加），所以收在这里。
 *
 * 行为：Left/Right（以及 Up/Down）在组内循环移动并选中；Home/End 到两端；
 * 整组只留一个 Tab 落点（选中项），焦点随选中项移动。
 */
export function useRovingSelection<T extends RovingOption>(opts: RovingSelectionOptions<T>) {
  /** 选中值不在选项里时退到第一项，否则整组都 Tab 不进去 */
  const activeValue = computed(() => {
    const list = opts.options()
    return list.some((o) => o.value === opts.current()) ? opts.current() : list[0]?.value
  })

  const btnRefs = ref<HTMLElement[]>([])
  function setBtnRef(el: unknown, index: number) {
    if (el instanceof HTMLElement) btnRefs.value[index] = el
  }

  function onKeydown(e: KeyboardEvent, index: number) {
    if (opts.disabled()) return
    const list = opts.options()
    const last = list.length - 1
    if (last < 0) return
    let next = -1
    switch (e.key) {
      case 'ArrowRight':
      case 'ArrowDown':
        next = index >= last ? 0 : index + 1
        break
      case 'ArrowLeft':
      case 'ArrowUp':
        next = index <= 0 ? last : index - 1
        break
      case 'Home':
        next = 0
        break
      case 'End':
        next = last
        break
      default:
        return
    }
    e.preventDefault()
    const option = list[next]
    if (!option) return
    if (option.value !== opts.current()) opts.select(option)
    void nextTick().then(() => btnRefs.value[next]?.focus())
  }

  return { activeValue, setBtnRef, onKeydown }
}
