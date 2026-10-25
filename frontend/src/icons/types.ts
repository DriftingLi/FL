/**
 * 图标数据类型：24 网格下的 `[tag, attrs]` 列表。
 *
 * 结构与 Lucide 的导出一致，也与 morphicons 的输入形状一致 —— 本仓只在**这里**声明一次，
 * 不把任一图标库的类型渗进调用方。换数据源时改的是注册表与这个文件，不是 177 处调用点。
 */
export type IconAttrs = Readonly<Record<string, string | number | undefined>>

/** 一个 24×24 图标：若干图形元素（path / circle / line / rect…）及其属性。 */
export type IconNode = ReadonlyArray<readonly [string, IconAttrs]>
