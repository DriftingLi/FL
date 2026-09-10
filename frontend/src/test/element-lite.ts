/**
 * epLite —— 测试专用的 ElementPlus 按需注册插件（#765 / #769）。
 *
 * 为什么存在：29 个 spec 全量 `plugins: [ElementPlus]`，import 阶段累计 310s
 * （全量 52s 里的大头）；CI 2 核 runner 上并发争抢导致 CourseCatalog 连续超时。
 *
 * 为什么从子路径 import：element-plus 的 package.json **没有 sideEffects 标注**，
 * `import { ElButton } from 'element-plus'` 走 barrel 无法 tree-shake，按需等于零。
 * 必须走 `element-plus/es/components/<name>/index.mjs` 子路径才是真按需。
 *
 * 白名单 = 全站模板实际用到的 49 个 el-* 组件（2026-09-10 扫描，含 ui/ 封装层内部
 * 依赖），兜底覆盖脚本扫描不到的动态用法。全站无 h(ElXxx) 动态渲染（实测 0 处），
 * 无 v-infinite-scroll；v-loading 34 处，随插件注册。
 *
 * 约定：**不做 stub、不做 shallowMount** —— 现有断言依赖 .el-* class 钩子与 EP
 * 真实交互行为。新 spec 请用 epLite 替代全量挂载（见 AGENTS.md「测试」词条）。
 */
import type { App, Component } from 'vue'

import ElAlert from 'element-plus/es/components/alert/index.mjs'
import ElAvatar from 'element-plus/es/components/avatar/index.mjs'
import ElBadge from 'element-plus/es/components/badge/index.mjs'
// 14 个「子组件」目录（button-group / form-item / option / table-column 等）只有
// style/（无 index.mjs），组件本体一律从父组件目录具名导出 —— 逐一验证过：
// collapse-item→collapse、descriptions-item→descriptions、dropdown-item/menu→dropdown、
// form-item→form、option/option-group→select、radio-button/radio-group→radio、
// tab-pane→tabs、table-column→table、button-group→button、checkbox-group→checkbox
import { ElButton, ElButtonGroup } from 'element-plus/es/components/button/index.mjs'
import { ElCheckbox, ElCheckboxGroup } from 'element-plus/es/components/checkbox/index.mjs'
import { ElRadio, ElRadioButton, ElRadioGroup } from 'element-plus/es/components/radio/index.mjs'
import ElCard from 'element-plus/es/components/card/index.mjs'
import ElCascader from 'element-plus/es/components/cascader/index.mjs'
import ElCol from 'element-plus/es/components/col/index.mjs'
import { ElCollapse, ElCollapseItem } from 'element-plus/es/components/collapse/index.mjs'
import ElConfigProvider from 'element-plus/es/components/config-provider/index.mjs'
import { ElDescriptions, ElDescriptionsItem } from 'element-plus/es/components/descriptions/index.mjs'
import ElDialog from 'element-plus/es/components/dialog/index.mjs'
import ElDivider from 'element-plus/es/components/divider/index.mjs'
import ElDrawer from 'element-plus/es/components/drawer/index.mjs'
import { ElDropdown, ElDropdownItem, ElDropdownMenu } from 'element-plus/es/components/dropdown/index.mjs'
import ElEmpty from 'element-plus/es/components/empty/index.mjs'
import { ElForm, ElFormItem } from 'element-plus/es/components/form/index.mjs'
import ElIcon from 'element-plus/es/components/icon/index.mjs'
import ElImage from 'element-plus/es/components/image/index.mjs'
import ElInput from 'element-plus/es/components/input/index.mjs'
import ElInputNumber from 'element-plus/es/components/input-number/index.mjs'
import ElLink from 'element-plus/es/components/link/index.mjs'
import ElPagination from 'element-plus/es/components/pagination/index.mjs'
import ElPopconfirm from 'element-plus/es/components/popconfirm/index.mjs'
import ElPopover from 'element-plus/es/components/popover/index.mjs'
import ElProgress from 'element-plus/es/components/progress/index.mjs'
import ElRow from 'element-plus/es/components/row/index.mjs'
import { ElOption, ElOptionGroup, ElSelect } from 'element-plus/es/components/select/index.mjs'
import ElSkeleton from 'element-plus/es/components/skeleton/index.mjs'
import ElSwitch from 'element-plus/es/components/switch/index.mjs'
import { ElTable, ElTableColumn } from 'element-plus/es/components/table/index.mjs'
import { ElTabPane, ElTabs } from 'element-plus/es/components/tabs/index.mjs'
import ElTag from 'element-plus/es/components/tag/index.mjs'
import ElTooltip from 'element-plus/es/components/tooltip/index.mjs'
import ElUpload from 'element-plus/es/components/upload/index.mjs'
import { vLoading } from 'element-plus/es/components/loading/index.mjs'

/** 白名单：全站实际用到的 49 个组件（含 button-group / config-provider 防意外） */
export const EP_LITE_WHITELIST = [
  ElAlert, ElAvatar, ElBadge, ElButton, ElButtonGroup, ElCard, ElCascader,
  ElCheckbox, ElCheckboxGroup, ElCol, ElCollapse, ElCollapseItem, ElConfigProvider,
  ElDescriptions, ElDescriptionsItem, ElDialog, ElDivider, ElDrawer,
  ElDropdown, ElDropdownItem, ElDropdownMenu, ElEmpty, ElForm, ElFormItem,
  ElIcon, ElImage, ElInput, ElInputNumber, ElLink, ElOption, ElOptionGroup,
  ElPagination, ElPopconfirm, ElPopover, ElProgress, ElRadio, ElRadioButton,
  ElRadioGroup, ElRow, ElSelect, ElSkeleton, ElSwitch, ElTabPane, ElTable,
  ElTableColumn, ElTabs, ElTag, ElTooltip, ElUpload
]

export interface EpLitePlugin {
  install(app: App): void
}

/**
 * 创建按需注册插件。
 * @param extra 额外组件（白名单之外的），调用方自行从 element-plus/es 子路径 import
 */
export function epLite(extra: Component[] = []): EpLitePlugin {
  const seen = new Set<string>()
  const components = ([...EP_LITE_WHITELIST, ...extra] as Component[]).filter((c) => {
    const name = (c as { name?: string }).name
    if (!name || seen.has(name)) return false
    seen.add(name)
    return true
  })
  return {
    install(app) {
      for (const c of components) {
        // EP 组件的复杂泛型签名（如 ElUpload 的构造器重载）与 app.component 的
        // 重载不完全匹配，运行时注册无差别 —— 类型层用宽断言收敛
        app.component((c as Component & { name: string }).name, c as never)
      }
      app.directive('loading', vLoading)
    }
  }
}
