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

/** 省级级联选项（省 → 市；直辖市叶子）。value 取 label（与存储契约一致）。 */
export function buildCityLevelRegionOptions(): RegionOption[] {
  return (pcTextArr || []).map((prov: any) => {
    const children = (prov.children || [])
      // 直辖市二级「市辖区/区」不展开为市级（#486 特判：直辖市只有一段）
      .filter((c: any) => c.label !== '市辖区')
      .map((c: any) => ({ label: c.label, value: c.label }))
    return {
      label: prov.label,
      value: prov.label,
      // 直辖市无 children（叶子）：北京/上海/天津/重庆
      ...(children.length ? { children } : {})
    }
  })
}

/** 按 / 拆分存储串为路径段（回显用；直辖市一段）。 */
export function splitRegionPath(region: string): string[] {
  if (!region) return []
  return region
    .split('/')
    .map((s) => s.trim())
    .filter(Boolean)
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
