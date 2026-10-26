#!/usr/bin/env node
/**
 * 一次性抽取：把 Element Plus 的 8 个**实心**图标转成 24 网格数据，写进 src/icons/filled.ts。
 *
 * 为什么要抽：Lucide 只有描边图标，而现网这 8 个承担「选中 / 强调」语义，换成描边会改语义重量。
 * 为什么不是直接搬 d：EP 图标画在 **1024×1024** 网格上，而本仓注册表统一 24 网格（形变也要求
 * 两端同坐标空间）。这里用 morphicons 的 fitIcon 做重网格（归一化成三次贝塞尔后按 24 重排），
 * 而不是手改坐标 —— 手改遇到圆弧必然出错。
 *
 * 注意：@element-plus/icons-vue 已从 dependencies 摘除（#1912）。重跑前先 npm i --no-save @element-plus/icons-vue。
 * 产物提交入库；本脚本留档以便复核（重跑：node scripts/extract-ep-filled.mjs，在 frontend/ 下）。
 * 来源：@element-plus/icons-vue 2.3.2（MIT），https://github.com/element-plus/element-plus-icons
 */
import { writeFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { createSSRApp, h } from 'vue'
import { renderToString } from 'vue/server-renderer'
import { fitIcon } from 'morphicons'
import {
  CircleCheckFilled,
  CircleCloseFilled,
  HomeFilled,
  InfoFilled,
  QuestionFilled,
  StarFilled,
  UserFilled,
  WarningFilled
} from '@element-plus/icons-vue'

const HERE = dirname(fileURLToPath(import.meta.url))

/** 注册表 key → EP 导出（顺序即导出顺序） */
const SOURCES = [
  ['star-filled', StarFilled],
  ['circle-check-filled', CircleCheckFilled],
  ['circle-x-filled', CircleCloseFilled],
  ['user-filled', UserFilled],
  ['house-filled', HomeFilled],
  ['info-filled', InfoFilled],
  ['circle-question-filled', QuestionFilled],
  ['triangle-alert-filled', WarningFilled]
]

const pathsOf = async (component) => {
  const html = await renderToString(createSSRApp({ render: () => h(component) }))
  return [...html.matchAll(/ d="([^"]+)"/g)].map((m) => m[1])
}

const lines = []
for (const [key, component] of SOURCES) {
  const raws = await pathsOf(component)
  const nodes = raws.map((d) => {
    const fitted = fitIcon([['path', { d }]], '0 0 1024 1024', 24)
    return "    ['path', { d: '" + fitted + "', fill: 'currentColor', stroke: 'none' }]"
  })
  lines.push('  /** ' + key + '（EP 1024 网格 → 24 重网格，生成物） */')
  lines.push("  '" + key + "': [")
  lines.push(nodes.join(',\n'))
  lines.push('  ],')
}

const body = [
  '/**',
  ' * 实心图标数据：**生成物，勿手改**（ADR-0079 / 票 #1912）。',
  ' *',
  ' * 生成命令：node scripts/extract-ep-filled.mjs（在 frontend/ 下执行）',
  ' * 来源：@element-plus/icons-vue 2.3.2（MIT）—— https://github.com/element-plus/element-plus-icons',
  ' * 变换：EP 画在 1024 网格，用 morphicons 的 fitIcon 归一到本仓统一的 24 网格；',
  ' *      每条路径带 fill: currentColor / stroke: none，渲染时不需要额外的 paint 开关。',
  ' */',
  "import type { IconNode } from './types'",
  '',
  'export const FILLED_ICONS = {',
  ...lines,
  '} as const satisfies Record<string, IconNode>',
  ''
].join('\n')

writeFileSync(resolve(HERE, '..', 'src', 'icons', 'filled.ts'), body)
console.log('written filled.ts, icons:', SOURCES.length)