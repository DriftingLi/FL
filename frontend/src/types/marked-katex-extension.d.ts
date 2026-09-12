/**
 * `marked-katex-extension` 的公开面声明（#901）。
 *
 * 为什么需要它：该包把自己的 **TypeScript 源码**当类型入口发布（package.json 的 `types`
 * 指向 `src/index.ts`），于是 vue-tsc 会连着它的源码一起检查——其中 `blockKatex(options, …)`
 * 有一个未使用的参数，在仓库的 `noUnusedParameters: true` 下直接报 TS6133，
 * `npm run type-check` 变红。上游自己的构建不在乎（它们的 tsconfig 没开这条）。
 *
 * 与其关掉全仓的 `noUnusedParameters`（拿掉一条真实的检查去迁就一个依赖），
 * 不如把该模块的**类型面显式接管**：tsconfig 的 `paths` 把这个模块名指到这里，
 * 声明我们真正用到的公开面。**运行时不受影响**——打包器仍解析到真包
 * （vite 不读 tsconfig 的 paths），类型声明不参与构建产物。
 *
 * 若哪天上游把类型入口改成正常的 `.d.ts`，删掉这里与 tsconfig 里的那条 paths 即可。
 */
import type { MarkedExtension } from 'marked'
import type { KatexOptions } from 'katex'

export interface MarkedKatexOptions extends KatexOptions {
  /** 非标准模式：不要求 `$...$` 右侧是空白或标点。本仓**不用**（见 ADR-0046「行内公式的边界」）。 */
  nonStandard?: boolean
}

declare const markedKatex: (options?: MarkedKatexOptions) => MarkedExtension
export default markedKatex
