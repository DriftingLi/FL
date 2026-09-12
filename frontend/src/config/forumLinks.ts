/**
 * 论坛内容相关的路径常量（ADR-0044）。
 *
 * 为什么单独抽出来：`ForumContent` 在 **AST 层**改写站外链接的 href，那里拿不到路由记录，
 * 只能写路径字面量；而路由表里是别名 + 相对子路径。两处一旦不同步，**所有站外链接会静默 404**
 * ——没有报错、没有类型提示。
 *
 * 故此处是唯一字面量来源，并由 `router/__tests__/linkOutRoute.spec.ts` 断言
 * 「该路径确实解析到 routeNames.LinkOut」，改名漏改时测试直接红。
 */
export const FORUM_LINK_OUT_PATH = "/training/link-out"
