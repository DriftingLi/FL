// 地区契约工具（#486）：市级两级级联选项与存储串路径操作。
// 数据契约与后端一致：意向地区与现居地存两段「省/市」中文串（直辖市一段）。
// 级联数据源：element-china-area-data 的 pcTextArr（省 → 市），
// 直辖市不再展开「市辖区」，即直辖市为叶子节点。
import { pcTextArr } from 'element-china-area-data'

export interface RegionOption {
  label: string
  value: string
  children?: RegionOption[]
}

// 直辖市名单：存储/展示只有一段（北京市/天津市/上海市/重庆市），二级区名不进入市级级联。
const MUNICIPALITY_NAMES = ['北京市', '天津市', '上海市', '重庆市']

/** 省级级联选项（省 → 市；直辖市叶子）。value 取 label（与存储契约一致）。 */
export function buildCityLevelRegionOptions(): RegionOption[] {
  return (pcTextArr || []).map((prov: any) => {
    // #486：直辖市按名单特判为叶子——pcTextArr 中其 children 是区名（非市），
    // 若展开会让学员存出「北京市/东城区」两段，回显与筛选契约（一段式）双双击穿。
    if (MUNICIPALITY_NAMES.includes(prov.label)) {
      return { label: prov.label, value: prov.label }
    }
    const children = (prov.children || []).map((c: any) => ({ label: c.label, value: c.label }))
    return {
      label: prov.label,
      value: prov.label,
      children
    }
  })
}

/** 按 / 拆分存储串为路径段（回显用；直辖市一段）。 */
export function splitRegionPath(region: string): string[] {
  if (!region) return []
  const parts = region
    .split('/')
    .map((s) => s.trim())
    .filter(Boolean)
  // #486：直辖市只有一段——存量若存「北京市/东城区」两段，回显只取直辖市节点（区不入级联路径）
  if (parts.length >= 2 && MUNICIPALITY_NAMES.includes(parts[0])) {
    return [parts[0]]
  }
  return parts
}

/** 路径段 → 存储串。 */
export function joinRegionPath(parts: string[]): string {
  return parts.filter(Boolean).join('/')
}

/** expected_regions 数组元素 → 级联路径数组（每个元素可能是「省/市」或直辖市一段）。 */
export function regionElementsToPaths(regions: string[]): string[][] {
  return (regions || []).map((r) => splitRegionPath(r)).filter((p) => p.length > 0)
}

/** 级联选择值数组 → 存储串数组（每项为「省/市」或直辖市一段）。 */
export function cascaderToRegionStrings(values: any[][]): string[] {
  return (values || [])
    .filter((v) => Array.isArray(v) && v.length > 0)
    .map((v) => joinRegionPath(v as string[]))
}
