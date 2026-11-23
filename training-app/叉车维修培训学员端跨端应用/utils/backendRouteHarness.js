/**
 * 后端 gin 路由清单扫描 —— 幻影路由锁的共用取值面（#2776）
 *
 * **只出事实、不做断言**（ADR-0023）：本文件不 `require('expect')`、不 `throw`，
 * 只把「handler.go 里注册了哪些路径」算成字符串数组。
 *
 * ## 为什么要有这一份
 *
 * 各契约测试原先自带一份 `/g\.(GET|POST|PUT|DELETE|PATCH)\("([^"]+)"/` —— **只认接收者叫 `g`**。
 * practicemode 把 `/free` `/tag` `/sequential` `/sequential-progress` `/progress(POST)` 挂在
 * `legacy := g.Group("", credentialScoped)` 上（`backend/internal/practicemode/handler.go:44-49`），
 * 于是这些**真存在**的路由被判成幻影，master 移动端接线腿自 #2773 起假红（本票来路）。
 *
 * 这里改成按 `X := Y.Group("<sub>")` 链解析前缀，接收者名叫什么都算、嵌套多深都算；
 * `attempts := rg.Group("/practice-mode/attempts")` 这种自带完整前缀的分组也不再由调用方猜。
 *
 * 返回值**去重**（同一路径可同时有 GET/POST，如 `/practice-mode/progress`）：调用方比的是路径集合。
 *
 * @param {string} src handler.go 源码
 * @returns {string[]} 已注册路径（含 group 前缀，去重，保持源码顺序）
 */
function scanRoutes(src) {
  const body = stripComments(String(src));

  const groups = new Map([['rg', '']]);
  const decls = [];
  const declRe = /([A-Za-z_$][\w$]*)\s*:=\s*([A-Za-z_$][\w$]*)\.Group\(\s*"([^"]*)"/g;
  let m;
  while ((m = declRe.exec(body)) !== null) decls.push({ name: m[1], parent: m[2], sub: m[3] });
  // 多趟解析：分组声明不保证父在子前
  let progressed = true;
  while (progressed) {
    progressed = false;
    for (const d of decls) {
      if (groups.has(d.name) || !groups.has(d.parent)) continue;
      groups.set(d.name, groups.get(d.parent) + d.sub);
      progressed = true;
    }
  }

  const out = [];
  const seen = new Set();
  const routeRe = /([A-Za-z_$][\w$]*)\.(GET|POST|PUT|DELETE|PATCH)\(\s*"([^"]*)"/g;
  while ((m = routeRe.exec(body)) !== null) {
    if (!groups.has(m[1])) continue;
    const p = groups.get(m[1]) + m[3];
    if (seen.has(p)) continue;
    seen.add(p);
    out.push(p);
  }
  return out;
}

/** 抹掉注释（`://` 例外，防误杀 URL 与 `// @Router` 之外的字面量） */
function stripComments(s) {
  return s.replace(/\/\*[\s\S]*?\*\//g, '').replace(/(^|[^:])\/\/[^\n]*/g, '$1');
}

module.exports = { scanRoutes };
