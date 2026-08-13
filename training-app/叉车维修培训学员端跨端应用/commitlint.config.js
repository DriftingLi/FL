/**
 * Commit 规范校验配置（Conventional Commits）
 * 状态：当前仓库未接入 CI，本文件为"就绪资产"，待接入 husky / CI 时启用。
 *
 * 启用方式（后续）：
 *   1) npm i -D @commitlint/cli @commitlint/config-conventional
 *   2) 接入 husky：npx husky add .husky/commit-msg "npx --no -- commitlint --edit \$1"
 *   3) 或在 CI 中加一步：npx commitlint --from=origin/main --to=HEAD --verbose
 *
 * 提交格式：<type>(<scope>): <subject>
 * 例：feat(exam): 新增答题计时器
 */
module.exports = {
  extends: ['@commitlint/config-conventional'],
  rules: {
    // type 必须是指定集合之一
    'type-enum': [
      2,
      'always',
      [
        'feat',     // 新功能
        'fix',      // 缺陷修复
        'refactor', // 重构（不改行为）
        'style',    // 格式/样式，无逻辑变更
        'docs',     // 文档
        'test',     // 测试
        'build',    // 构建/依赖
        'ci',       // CI 流程
        'chore',    // 杂项（谨慎使用，禁止"代码清洗"类模糊描述）
        'perf',     // 性能优化
        'revert',   // 回滚
      ],
    ],
    // scope 推荐但非强制
    'scope-case': [0],
    // subject 小写开头、祈使句、≤ 50 字、不加句号
    'subject-case': [2, 'always', 'lower-case'],
    'subject-empty': [2, 'never'],
    'subject-full-stop': [2, 'never', '.'],
    'header-max-length': [2, 'always', 72],
    // 禁止无意义的 chore 描述
    'scope-enum': [0],
  },
};
