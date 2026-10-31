import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

/**
 * 封装层的类名合并单点（#2156 变体批）。
 *
 * - `clsx` 收条件类名（对象 / 数组 / 假值），
 * - `tailwind-merge` 消解**同属性冲突**：调用方传 `p-6` 时稳定压过组件内的 `p-4`，
 *   不必再写 `!` 或 `:deep()` 去抢优先级。
 *
 * 只给封装层内部用；业务模板继续写原子类（R1/R3 不变）。
 */
export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs))
}
