"""未匹配待审队列的列名契约与视图口径的回归钉（#2676）。

TDD：先于实现写下，钉住 #2676 的判定口径。口径的**唯一真相是迁移 000067 的
`market_unmatched_queue` 视图**，本文件只验列名契约与视图无关的静态约定；
真正的聚合/排除/排序行为由 `test_e2e_contract.py` 对真 PG 逐条断言
（本机无 DATABASE_URL 时那些测试干净 skip，首跑在 CI 的 collector-test job）。
"""

import unittest

from collector.review import UNMATCHED_GROUP_COLUMNS


class GroupColumnsTest(unittest.TestCase):
    def test_group_columns_are_stable(self):
        """列名固定：视图列、store 的 SELECT、CLI 表头共用同一份定义。"""
        self.assertEqual(
            UNMATCHED_GROUP_COLUMNS,
            ("brand", "model", "tonnage", "sample_count", "price_min", "price_max"),
        )

    def test_group_columns_match_migration_view(self):
        """列名契约必须与迁移 000067 视图的列一一对应（改一处就要改另一处）。"""
        from pathlib import Path

        sql = next(
            Path(__file__).resolve().parents[2].glob(
                "backend/migrations/000067_*.up.sql"
            )
        ).read_text(encoding="utf-8")
        for column in UNMATCHED_GROUP_COLUMNS:
            self.assertIn(f"AS {column}", sql, f"迁移视图缺少列 {column}")


if __name__ == "__main__":
    unittest.main()
