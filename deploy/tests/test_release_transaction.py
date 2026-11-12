"""ReleaseTransaction module 测试：plan 构造、execute、rollback、verify。

seam = Environment interface（注入 fake，不碰 Docker）。
运行：python3 -m unittest discover -s deploy/tests -p 'test_*.py'
"""
from __future__ import annotations

import sys
import unittest
from pathlib import Path
from typing import Dict, List, Optional

# 动态导入 release_transaction（与 test_deploy.py 同一方式）
_DEPLOY_DIR = Path(__file__).resolve().parent.parent
if str(_DEPLOY_DIR) not in sys.path:
    sys.path.insert(0, str(_DEPLOY_DIR))

from release_transaction import (  # noqa: E402
    ReleaseTransaction,
    build_release_plan,
)


# ---------------------------------------------------------------- fake 环境
class FakeEnvironment:
    """记录所有调用顺序，供测试断言。"""

    def __init__(self, running_images: Optional[Dict[str, str]] = None):
        self.running_images = running_images or {}
        self.calls: List[tuple] = []
        self.migration_preflight_ok = True
        self.verify_fail = False  # 设为 True 时 verify_running_image 返回 False

    def get_running_image(self, service: str) -> str:
        self.calls.append(("get_running_image", service))
        return self.running_images.get(service, "unknown")

    def recreate_service(self, service: str) -> None:
        self.calls.append(("recreate_service", service))

    def set_service_image(self, service: str, image: str) -> None:
        self.calls.append(("set_service_image", service, image))

    def verify_running_image(self, service: str, expected: str) -> bool:
        self.calls.append(("verify_running_image", service, expected))
        return not self.verify_fail

    def preflight_migration_check(self, image: str) -> bool:
        self.calls.append(("preflight_migration_check", image))
        return self.migration_preflight_ok

    def fix_dirty_state(self) -> None:
        self.calls.append(("fix_dirty_state",))

    def run_migration(self) -> None:
        self.calls.append(("run_migration",))

    def health_check(self) -> None:
        self.calls.append(("health_check",))


# ---------------------------------------------------------------- plan 构造
class TestBuildReleasePlan(unittest.TestCase):
    """plan 构造：含 libreoffice、每服务有 previous/target 双状态。"""

    def test_plan_includes_libreoffice_when_enabled(self):
        """libreoffice 启用时必须进 plan（修复快照/回滚缺失缺陷）。"""
        env = FakeEnvironment(running_images={
            "backend": "forklift-backend:v1",
            "frontend": "forklift-frontend:v1",
            "libreoffice": "forklift-libreoffice:v1",
        })
        plan = build_release_plan(
            env=env,
            desired_images={
                "backend": "forklift-backend:v2",
                "frontend": "forklift-frontend:v2",
                "libreoffice": "forklift-libreoffice:v2",
            },
            decisions=None,
            migration_needed=True,
        )
        services = {s.service: s for s in plan.services}
        self.assertIn("libreoffice", services, "libreoffice 必须在 plan 中")
        self.assertEqual(services["libreoffice"].previous_image, "forklift-libreoffice:v1")
        self.assertEqual(services["libreoffice"].target_image, "forklift-libreoffice:v2")

    def test_plan_excludes_libreoffice_when_disabled(self):
        """libreoffice 未启用时不进 plan。"""
        env = FakeEnvironment()
        plan = build_release_plan(
            env=env,
            desired_images={"backend": "b:v2", "frontend": "f:v2"},
            decisions=None,
            migration_needed=False,
        )
        services = {s.service for s in plan.services}
        self.assertNotIn("libreoffice", services)

    def test_plan_previous_image_unknown_when_not_running(self):
        """未运行的服务 previous_image 为 'unknown'。"""
        env = FakeEnvironment(running_images={})
        plan = build_release_plan(
            env=env,
            desired_images={"backend": "b:v2"},
            decisions=None,
            migration_needed=False,
        )
        self.assertEqual(plan.services[0].previous_image, "unknown")

    def test_plan_records_migration_needed(self):
        """plan 记录是否需要迁移。"""
        env = FakeEnvironment()
        plan = build_release_plan(
            env=env, desired_images={}, decisions=None, migration_needed=True,
        )
        self.assertTrue(plan.migration_needed)


# ---------------------------------------------------------------- execute
class TestExecute(unittest.TestCase):
    """execute：按计划推进，记录 applied。"""

    def test_execute_applies_changed_services(self):
        """只 apply action != skip 的服务。"""
        env = FakeEnvironment(running_images={"backend": "b:v1", "frontend": "f:v1"})
        plan = build_release_plan(
            env=env,
            desired_images={"backend": "b:v2", "frontend": "f:v1"},
            decisions=None,
            migration_needed=False,
        )
        txn = ReleaseTransaction(plan, env)
        txn.execute()
        recreate_calls = [c for c in env.calls if c[0] == "recreate_service"]
        self.assertEqual(len(recreate_calls), 1)
        self.assertEqual(recreate_calls[0][1], "backend")

    def test_execute_runs_migration_when_needed(self):
        """migration_needed=True 时执行 fix_dirty + migrate。"""
        env = FakeEnvironment()
        plan = build_release_plan(
            env=env, desired_images={}, decisions=None, migration_needed=True,
        )
        txn = ReleaseTransaction(plan, env)
        txn.execute()
        call_names = [c[0] for c in env.calls]
        self.assertIn("fix_dirty_state", call_names)
        self.assertIn("run_migration", call_names)

    def test_execute_skips_migration_when_not_needed(self):
        """migration_needed=False 时不执行迁移。"""
        env = FakeEnvironment()
        plan = build_release_plan(
            env=env, desired_images={}, decisions=None, migration_needed=False,
        )
        txn = ReleaseTransaction(plan, env)
        txn.execute()
        call_names = [c[0] for c in env.calls]
        self.assertNotIn("run_migration", call_names)

    def test_execute_calls_health_check(self):
        """execute 末尾调 health_check。"""
        env = FakeEnvironment()
        plan = build_release_plan(
            env=env, desired_images={}, decisions=None, migration_needed=False,
        )
        txn = ReleaseTransaction(plan, env)
        txn.execute()
        self.assertEqual(env.calls[-1], ("health_check",))


# ---------------------------------------------------------------- rollback
class TestRollback(unittest.TestCase):
    """rollback：基于同一 plan 反转，只动 changed 服务。"""

    def test_rollback_restores_previous_images(self):
        """rollback 将已 apply 的服务恢复到 previous_image（含 libreoffice）。"""
        env = FakeEnvironment(running_images={"backend": "b:v1", "libreoffice": "lo:v1"})
        plan = build_release_plan(
            env=env,
            desired_images={"backend": "b:v2", "libreoffice": "lo:v2"},
            decisions=None,
            migration_needed=False,
        )
        txn = ReleaseTransaction(plan, env)
        txn.execute()
        env.calls.clear()
        txn.rollback()
        # 验证 set_service_image + recreate_service 成对出现
        set_calls = [c for c in env.calls if c[0] == "set_service_image"]
        recreate_calls = [c for c in env.calls if c[0] == "recreate_service"]
        self.assertEqual(len(set_calls), 2)
        self.assertEqual(len(recreate_calls), 2)
        # set_service_image 在 recreate_service 之前
        for i, sc in enumerate(set_calls):
            rc_idx = env.calls.index(("recreate_service", sc[1]))
            sc_idx = env.calls.index(sc)
            self.assertLess(sc_idx, rc_idx, f"set_service_image({sc[1]}) 应在 recreate 之前")
        services_rolled_back = {c[1] for c in recreate_calls}
        self.assertIn("backend", services_rolled_back)
        self.assertIn("libreoffice", services_rolled_back)

    def test_rollback_uses_reversed_order(self):
        """rollback 按反转序执行：后部署的先回滚。"""
        env = FakeEnvironment(running_images={
            "backend": "b:v1", "frontend": "f:v1", "libreoffice": "lo:v1",
        })
        plan = build_release_plan(
            env=env,
            desired_images={
                "backend": "b:v2", "frontend": "f:v2", "libreoffice": "lo:v2",
            },
            decisions=None,
            migration_needed=False,
        )
        txn = ReleaseTransaction(plan, env)
        txn.execute()  # apply 顺序：backend → frontend → libreoffice（按 plan 序）
        env.calls.clear()
        txn.rollback()  # 应反转：libreoffice → frontend → backend
        recreate_calls = [c for c in env.calls if c[0] == "recreate_service"]
        rollback_order = [c[1] for c in recreate_calls]
        # plan 顺序是 backend → frontend → libreoffice，rollback 应反转
        self.assertEqual(rollback_order, ["libreoffice", "frontend", "backend"])

    def test_rollback_only_touches_applied_services(self):
        """rollback 只动 execute 中实际 apply 过的服务。"""
        env = FakeEnvironment(running_images={"backend": "b:v1", "frontend": "f:v1"})
        plan = build_release_plan(
            env=env,
            desired_images={"backend": "b:v2", "frontend": "f:v1"},
            decisions=None,
            migration_needed=False,
        )
        txn = ReleaseTransaction(plan, env)
        txn.execute()
        env.calls.clear()
        txn.rollback()
        recreate_calls = [c for c in env.calls if c[0] == "recreate_service"]
        self.assertEqual(len(recreate_calls), 1)
        self.assertEqual(recreate_calls[0][1], "backend")

    def test_rollback_with_no_applied_is_noop(self):
        """没 apply 过任何服务时 rollback 不做事。"""
        env = FakeEnvironment()
        plan = build_release_plan(
            env=env, desired_images={}, decisions=None, migration_needed=False,
        )
        txn = ReleaseTransaction(plan, env)
        txn.rollback()
        self.assertFalse(any(c[0] == "recreate_service" for c in env.calls))


# ---------------------------------------------------------------- verify
class TestVerify(unittest.TestCase):
    """verify：消费同一 plan 校验运行镜像。"""

    def test_verify_checks_all_applied_services(self):
        """verify 校验所有 apply 过的服务运行镜像 = target_image。"""
        env = FakeEnvironment(running_images={"backend": "b:v1"})
        plan = build_release_plan(
            env=env,
            desired_images={"backend": "b:v2"},
            decisions=None,
            migration_needed=False,
        )
        txn = ReleaseTransaction(plan, env)
        txn.execute()
        env.calls.clear()
        txn.verify()
        verify_calls = [c for c in env.calls if c[0] == "verify_running_image"]
        self.assertEqual(len(verify_calls), 1)
        self.assertEqual(verify_calls[0][1], "backend")
        self.assertEqual(verify_calls[0][2], "b:v2")

    def test_verify_raises_on_mismatch(self):
        """verify 发现镜像不匹配时 raise RuntimeError。"""
        env = FakeEnvironment(running_images={"backend": "b:v1"})
        plan = build_release_plan(
            env=env,
            desired_images={"backend": "b:v2"},
            decisions=None,
            migration_needed=False,
        )
        txn = ReleaseTransaction(plan, env)
        txn.execute()
        env.verify_fail = True
        with self.assertRaises(RuntimeError):
            txn.verify()


if __name__ == "__main__":
    unittest.main()
