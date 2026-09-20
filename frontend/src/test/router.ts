/**
 * testRouter —— 测试用的最小 vue-router 实例（ADR-0060 票8b）。
 *
 * 为什么存在：跳转改走 `href('Name')` 之后，落点是**具名位置**，必须有对应路由记录才解析得动；
 * 而各 spec 原先手写的 `routes: [{ path: '/:pathMatch(.*)*' }]` 通配记录只吃路径、不吃名字
 * （装具名位置即 `No match for {"name":"StudentProfile"}`）。
 *
 * 为什么路由表从 `config/pages.ts` 派生而不是再手抄一份：描述符表是 name 与 path 的唯一事实源，
 * spec 里再列一遍就是本票要消灭的那种第二份清单——页面改路径时它会静默漂移。
 *
 * 组件一律换成空壳：这里测的是「路由解析 + 落点渲染」，不是懒加载进来的页面本体。
 */
import { createRouter, createMemoryHistory, type Router } from 'vue-router'
import { pages } from '@/config/pages'

const BLANK = { template: '<div/>' }

/** 覆盖全部描述符页面的测试路由（每条一个空壳组件，名字与路径原样取自描述符表）。 */
export function testRouter(): Router {
  return createRouter({
    history: createMemoryHistory(),
    routes: pages.map(page => ({ path: page.path, name: page.name, component: BLANK }))
  })
}
