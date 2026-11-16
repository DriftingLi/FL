/**
 * pages.json 已注册路由（拍平 pages ∪ subPackages）。
 *
 * #2579 起非 tabBar 页整目录切进普通分包，分包条目的 path 是相对各自 root 的短名
 * ⇒ 只读 pages 段、或对原文做 `"path": "pages/x/y"` 子串匹配，都会把分包页误判成未注册。
 * 判据只有一份：这里拍平出完整路由（root + '/' + path），调用方按完整路由断言。
 */
function registeredRoutes(pagesJsonText) {
  const conf = typeof pagesJsonText === 'string' ? JSON.parse(pagesJsonText) : pagesJsonText;
  const main = (conf.pages || []).map((p) => p.path);
  const sub = (conf.subPackages || []).flatMap((s) => (s.pages || []).map((p) => `${s.root}/${p.path}`));
  return [...main, ...sub];
}

module.exports = { registeredRoutes };
