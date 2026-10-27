"""deploy/deploy.py 的纯判定单元测试（不碰 docker / 不写部署文件）。

跑法：python3 -m unittest discover -s deploy/tests -p 'test_*.py' -v
CI 在 backend-test 之后单独跑这一步；Node 的两个 dry-run 判据物在 deploy/tests/*.test.mjs。
"""

from __future__ import annotations

import importlib.util
import os
import sys
import unittest
from pathlib import Path

HERE = Path(__file__).resolve().parent
MODULE_PATH = HERE.parent / "deploy.py"
spec = importlib.util.spec_from_file_location("deploy_cli", MODULE_PATH)
assert spec and spec.loader
deploy = importlib.util.module_from_spec(spec)
sys.modules["deploy_cli"] = deploy
spec.loader.exec_module(deploy)


class EnvDefaultsTest(unittest.TestCase):
    def setUp(self):
        self._saved = dict(os.environ)
        deploy.set_defaults({})

    def tearDown(self):
        os.environ.clear()
        os.environ.update(self._saved)
        deploy.set_defaults({})

    def test_parse_env_defaults_shell_quotes(self):
        got = deploy.parse_env_defaults(
            "# c\nexport FOO='a b'\nexport CN='和润天下'\nexport EMPTY=''\n"
        )
        self.assertEqual(got, {"FOO": "a b", "CN": "和润天下", "EMPTY": ""})

    def test_defaults_never_override_real_environment(self):
        deploy.set_defaults({"PG_VOLUME": "pgdata-prod", "DB_USER": "forklift"})
        os.environ["PG_VOLUME"] = "/srv/ceph/pgdata"
        os.environ.pop("DB_USER", None)
        self.assertEqual(deploy.getenv("PG_VOLUME"), "/srv/ceph/pgdata")
        self.assertEqual(deploy.getenv("DB_USER"), "forklift")
        # 关键：默认值只进内存，不进 os.environ（否则 compose 子进程会用它盖掉 .env）。
        self.assertNotIn("DB_USER", os.environ)
        self.assertEqual(os.environ["PG_VOLUME"], "/srv/ceph/pgdata")

    def test_env_val(self):
        self.assertEqual(deploy.env_val(""), '""')
        self.assertEqual(deploy.env_val("abc"), "abc")
        self.assertEqual(deploy.env_val("a b"), '"a b"')
        self.assertEqual(deploy.env_val("a\nb"), '"a\\nb"')
        self.assertEqual(deploy.env_val('a"b'), '"a\\"b"')
        self.assertEqual(deploy.env_val("a\\b"), '"a\\\\b"')

class DecideServiceTest(unittest.TestCase):
    def state(self, running=True, image="img:a", image_id="sha256:1", config_hash="h1"):
        return deploy.ServiceState(running=running, image=image, image_id=image_id, config_hash=config_hash)

    def test_missing_creates(self):
        d = deploy.decide_service("backend", "img:a", None, "h1")
        self.assertEqual(d.action, "create")

    def test_stopped_starts(self):
        d = deploy.decide_service("backend", "img:a", self.state(running=False), "h1")
        self.assertEqual(d.action, "start")

    def test_unchanged_skips(self):
        d = deploy.decide_service("backend", "img:a", self.state(), "h1", "sha256:1")
        self.assertEqual(d.action, "skip")

    def test_image_change_recreates(self):
        d = deploy.decide_service("backend", "img:b", self.state(), "h1", "sha256:2")
        self.assertEqual(d.action, "recreate")
        self.assertIn("镜像变更", d.reason)

    def test_same_image_id_different_tag_skips(self):
        d = deploy.decide_service("backend", "mirror/img:a", self.state(image="ghcr.io/img:a"),
                                  "h1", "sha256:1")
        self.assertEqual(d.action, "skip")

    def test_same_tag_new_image_id_recreates(self):
        d = deploy.decide_service("backend", "img:a", self.state(image="img:a", image_id="sha256:old"),
                                  "h1", "sha256:new")
        self.assertEqual(d.action, "recreate")
        self.assertIn("镜像内容变更", d.reason)

    def test_config_drift_recreates_app(self):
        d = deploy.decide_service("backend", "img:a", self.state(config_hash="old"), "new", "sha256:1")
        self.assertEqual(d.action, "recreate")
        self.assertIn("配置变更", d.reason)

    def test_data_config_drift_blocks(self):
        d = deploy.decide_service("postgres", "", self.state(config_hash="old"), "new", data=True)
        self.assertEqual(d.action, "blocked")
        d = deploy.decide_service("postgres", "", self.state(config_hash="old"), "new", data=True,
                                  force_data=True)
        self.assertEqual(d.action, "recreate")

    def test_force_does_not_touch_data(self):
        d = deploy.decide_service("postgres", "", self.state(), "h1", data=True, force=True)
        self.assertEqual(d.action, "skip")

    def test_force_recreates_app(self):
        d = deploy.decide_service("backend", "img:a", self.state(), "h1", "sha256:1", force=True)
        self.assertEqual(d.action, "recreate")
        self.assertIn("--force", d.reason)

    def test_plan_order(self):
        states = {
            "postgres": self.state(),
            "redis": self.state(),
            "backend": self.state(image="img:old", image_id="sha256:old"),
            "frontend": self.state(image="img:new", image_id="sha256:new"),
        }
        decisions = deploy.plan_decisions(
            {"backend": "img:new", "frontend": "img:new"},
            states,
            {"postgres": "h1", "redis": "h1", "backend": "h1", "frontend": "h1"},
            {"backend": "sha256:new", "frontend": "sha256:new"},
        )
        self.assertEqual([d.service for d in decisions], ["postgres", "redis", "backend", "frontend"])
        self.assertEqual([d.action for d in decisions], ["skip", "skip", "recreate", "skip"])


class PureFunctionsTest(unittest.TestCase):
    def test_migration_failure_action(self):
        self.assertEqual(deploy.migration_failure_action(None), "abort")
        self.assertEqual(deploy.migration_failure_action(""), "abort")
        self.assertEqual(deploy.migration_failure_action("0"), "abort")
        self.assertEqual(deploy.migration_failure_action("true"), "abort")
        self.assertEqual(deploy.migration_failure_action("1"), "continue")

    def test_rollback_tag(self):
        self.assertEqual(deploy.rollback_tag("ghcr.io/o/app:sha-old"), "sha-old")
        self.assertEqual(deploy.rollback_tag("registry.local:5000/o/app:v1"), "v1")
        self.assertEqual(deploy.rollback_tag("ghcr.io/o/app"), "")
        self.assertEqual(deploy.rollback_tag("unknown"), "")
        self.assertEqual(deploy.rollback_tag(""), "")

    def test_images_equal(self):
        self.assertTrue(deploy.images_equal("a:b", "a:b"))
        self.assertFalse(deploy.images_equal("a:b", "a:c"))
        self.assertTrue(deploy.images_equal(None, None))

    def test_resolve_image(self):
        deploy.set_defaults({"BACKEND_IMAGE": "forklift-backend:latest"})
        try:
            self.assertEqual(deploy.resolve_image("IMAGE_BACKEND", "BACKEND_IMAGE", "c123"),
                             "forklift-backend:c123")
            os.environ["IMAGE_BACKEND"] = "GHCR.IO/ORG/FL-BACKEND"
            self.assertEqual(deploy.resolve_image("IMAGE_BACKEND", "BACKEND_IMAGE", "c123"),
                             "ghcr.io/org/fl-backend:c123")
        finally:
            os.environ.pop("IMAGE_BACKEND", None)
            deploy.set_defaults({})


class RenderEnvFileTest(unittest.TestCase):
    def test_tls_mode_and_images_are_written(self):
        deploy.set_defaults({
            "DB_USER": "forklift",
            "DOMAIN": "example.com",
            "PG_VOLUME": "pgdata-prod",
            "COMPOSE_PROJECT_NAME": "forklift-training",
        })
        try:
            d = deploy.Deployer.__new__(deploy.Deployer)
            d.db_password = "p@ss word"
            d.backend_ref = "ghcr.io/o/backend:c1"
            d.frontend_ref = "ghcr.io/o/frontend:c2"
            d.libreoffice_ref = "ghcr.io/o/lo:c3"
            out = d.render_env_file()
        finally:
            deploy.set_defaults({})
        self.assertIn("TLS_MODE=auto", out)
        self.assertIn("BACKEND_IMAGE=ghcr.io/o/backend:c1", out)
        self.assertIn("DB_PASSWORD=\"p@ss word\"", out)
        self.assertIn("COMPOSE_PROJECT_NAME=forklift-training", out)


if __name__ == "__main__":
    unittest.main()
