"""DeployerEnvironment adapter：Deployer → Environment Protocol 桥接测试。

验证 ReleaseTransaction 通过 Environment 接口操作 Deployer，
消除 deploy.py 中平行的 mutable 状态编排。
"""
from __future__ import annotations

import importlib.util
import sys
import unittest
from pathlib import Path
from types import SimpleNamespace

HERE = Path(__file__).resolve().parent
MODULE_PATH = HERE.parent / "deploy.py"
spec = importlib.util.spec_from_file_location("deploy_cli", MODULE_PATH)
assert spec and spec.loader
deploy_mod = importlib.util.module_from_spec(spec)
sys.modules["deploy_cli"] = deploy_mod
spec.loader.exec_module(deploy_mod)

sys.path.insert(0, str(HERE.parent))
from release_transaction import ReleaseTransaction, build_release_plan


def _make_deployer(**overrides):
    """构造最小 Deployer 实例（绕过 __init__），与 test_deploy.py 同模式。"""
    d = deploy_mod.Deployer.__new__(deploy_mod.Deployer)
    d.opts = SimpleNamespace(force=False, remove_orphans=False)
    d.applied = []
    d.compose_project_args = []
    d.profiles = []
    d.mode = "deploy"
    d.compose_file = Path("/tmp/compose.yml")
    d.env_file = Path("/tmp/.env")
    d.backend_ref = "forklift-backend:v2"
    d.frontend_ref = "forklift-frontend:v2"
    d.libreoffice_ref = "forklift-libreoffice:v2"
    d.libreoffice_enabled = True
    d._compose = lambda: ["docker", "compose"]
    d.previous_images = {}
    for k, v in overrides.items():
        setattr(d, k, v)
    return d


class DeployerEnvironmentTest(unittest.TestCase):
    """Environment Protocol 各方法委托到 Deployer。"""

    def _make_env(self, d):
        return deploy_mod.DeployerEnvironment(d)

    def test_get_running_image_delegates_to_inspect(self):
        d = _make_deployer()
        calls = []

        def run(cmd, **kwargs):
            calls.append(cmd)
            return SimpleNamespace(returncode=0, stdout="forklift-backend:v1\n", stderr="")

        d.run = run
        d.compose_ps_id = lambda svc, **kw: "abc123"
        env = self._make_env(d)
        result = env.get_running_image("backend")
        self.assertEqual(result, "forklift-backend:v1")

    def test_get_running_image_unknown_when_not_running(self):
        d = _make_deployer()
        d.compose_ps_id = lambda svc, **kw: ""
        env = self._make_env(d)
        result = env.get_running_image("backend")
        self.assertEqual(result, "unknown")

    def test_recreate_service_calls_compose_up(self):
        d = _make_deployer()
        calls = []

        def run(cmd, **kwargs):
            calls.append(cmd)
            return SimpleNamespace(returncode=0, stdout="", stderr="")

        d.run = run
        d._tail_logs = lambda svc: None
        env = self._make_env(d)
        env.recreate_service("backend")
        self.assertEqual(len(calls), 1)
        self.assertEqual(calls[0][-1], "backend")

    def test_set_service_image_updates_ref_and_writes_env(self):
        d = _make_deployer()
        written = []

        def write_env_file():
            written.append(True)

        d.write_env_file = write_env_file
        env = self._make_env(d)
        env.set_service_image("backend", "forklift-backend:v1")
        self.assertEqual(d.backend_ref, "forklift-backend:v1")
        self.assertEqual(written, [True])

    def test_verify_running_image_delegates(self):
        d = _make_deployer()
        d.assert_running_image = lambda svc, exp: svc == "backend" and exp == "v2"
        env = self._make_env(d)
        self.assertTrue(env.verify_running_image("backend", "v2"))
        self.assertFalse(env.verify_running_image("frontend", "v2"))

    def test_preflight_migration_check_delegates(self):
        d = _make_deployer()
        d.preflight_migration_check = lambda img: img == "ok-image"
        env = self._make_env(d)
        self.assertTrue(env.preflight_migration_check("ok-image"))
        self.assertFalse(env.preflight_migration_check("bad-image"))

    def test_fix_dirty_state_delegates(self):
        d = _make_deployer()
        called = []
        d.fix_dirty_state = lambda: called.append(True)
        env = self._make_env(d)
        env.fix_dirty_state()
        self.assertEqual(called, [True])

    def test_run_migration_delegates(self):
        d = _make_deployer()
        called = []
        d.run_migration = lambda: called.append(True)
        env = self._make_env(d)
        env.run_migration()
        self.assertEqual(called, [True])

    def test_health_check_delegates(self):
        d = _make_deployer()
        called = []
        d.health_check = lambda: called.append(True)
        env = self._make_env(d)
        env.health_check()
        self.assertEqual(called, [True])


class TransactionWithDeployerEnvTest(unittest.TestCase):
    """ReleaseTransaction 通过 DeployerEnvironment 驱动 Deployer。"""

    def test_execute_uses_deployer_methods(self):
        """txn.execute() 应通过 Environment 调用 Deployer 的迁移/重建/健康检查。"""
        d = _make_deployer()
        calls = []

        def run(cmd, **kwargs):
            calls.append(("run", cmd))
            return SimpleNamespace(returncode=0, stdout="forklift-backend:v1\n", stderr="")

        d.run = run
        d.compose_ps_id = lambda svc, **kw: "abc123" if svc == "backend" else ""
        d._tail_logs = lambda svc: None
        d.write_env_file = lambda: calls.append(("write_env",))
        d.preflight_migration_check = lambda img: True
        d.fix_dirty_state = lambda: calls.append(("fix_dirty",))
        d.run_migration = lambda: calls.append(("migrate",))
        d.health_check = lambda: calls.append(("health",))
        d.assert_running_image = lambda svc, exp: True

        env = deploy_mod.DeployerEnvironment(d)

        plan = build_release_plan(
            env=env,
            desired_images={"backend": "forklift-backend:v2"},
            decisions=None,
            migration_needed=True,
        )
        txn = ReleaseTransaction(plan, env)
        txn.execute()

        # 迁移先于服务变更
        fix_idx = next(i for i, c in enumerate(calls) if c[0] == "fix_dirty")
        migrate_idx = next(i for i, c in enumerate(calls) if c[0] == "migrate")
        run_idx = next(i for i, c in enumerate(calls) if c[0] == "run" and "up" in c[1])
        self.assertLess(fix_idx, run_idx)
        self.assertLess(migrate_idx, run_idx)

        # 健康检查在最后
        health_idx = next(i for i, c in enumerate(calls) if c[0] == "health")
        self.assertGreater(health_idx, run_idx)

    def test_rollback_restores_images(self):
        """txn.rollback() 应恢复 previous_image 并重建。"""
        d = _make_deployer()
        calls = []

        def run(cmd, **kwargs):
            calls.append(("run", cmd))
            return SimpleNamespace(returncode=0, stdout="forklift-backend:v1\n", stderr="")

        d.run = run
        d.compose_ps_id = lambda svc, **kw: "abc123" if svc == "backend" else ""
        d._tail_logs = lambda svc: None
        d.write_env_file = lambda: calls.append(("write_env",))
        d.preflight_migration_check = lambda img: True
        d.fix_dirty_state = lambda: calls.append(("fix_dirty",))
        d.run_migration = lambda: calls.append(("migrate",))
        d.health_check = lambda: calls.append(("health",))
        d.assert_running_image = lambda svc, exp: True

        env = deploy_mod.DeployerEnvironment(d)

        plan = build_release_plan(
            env=env,
            desired_images={"backend": "forklift-backend:v2"},
            decisions=None,
            migration_needed=False,
        )
        txn = ReleaseTransaction(plan, env)
        txn.execute()
        calls.clear()
        txn.rollback()

        # set_service_image 应先于 recreate_service
        set_idx = next(i for i, c in enumerate(calls) if c[0] == "write_env")
        run_idx = next(i for i, c in enumerate(calls) if c[0] == "run" and "up" in c[1])
        self.assertLess(set_idx, run_idx)


if __name__ == "__main__":
    unittest.main()
