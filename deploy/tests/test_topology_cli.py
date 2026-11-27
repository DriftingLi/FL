"""部署拓扑在 deploy CLI、发布与回滚链路中的测试。"""
from __future__ import annotations

import importlib.util
import io
import json
import os
import sys
import tempfile
import unittest
from contextlib import redirect_stdout
from pathlib import Path
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

from topology import read_topology_snapshot, snapshot_path_for

DEPLOY_MODULE_PATH = Path(__file__).resolve().parent.parent / "deploy.py"
_SPEC = importlib.util.spec_from_file_location("deploy_cli_topology_test", DEPLOY_MODULE_PATH)
assert _SPEC and _SPEC.loader
deploy_cli = importlib.util.module_from_spec(_SPEC)
sys.modules[_SPEC.name] = deploy_cli
_SPEC.loader.exec_module(deploy_cli)


class DeployCliTopologyTest(unittest.TestCase):
    def _environment(self, tmp: Path, mode: str):
        environment = {
            "DEPLOY_PATH": str(tmp),
            "COMPOSE_PROJECT_NAME": "forklift-training",
            "IMAGE_TAG": "sha-cli",
            "DB_MODE": mode,
        }
        if mode == "external":
            environment["DATABASE_URL"] = (
                "postgresql://forklift:local%26test@pg.internal/forklift_training"
                "?sslmode=verify-full&sslrootcert=/etc/forklift/postgres-ca.crt"
            )
        return environment

    def test_cli_topology_json_resolves_both_adapters_without_docker(self):
        for mode in ("docker", "external"):
            with self.subTest(mode=mode), tempfile.TemporaryDirectory() as raw_tmp:
                output = io.StringIO()
                with patch.dict(os.environ, self._environment(Path(raw_tmp), mode), clear=True):
                    with redirect_stdout(output):
                        result = deploy_cli.main(["--topology"])
                self.assertEqual(result, 0)
                payload = json.loads(output.getvalue())
                self.assertEqual(payload["release_id"], "sha-cli")
                self.assertEqual(payload["database_mode"], mode)
                names = [Path(item).name for item in payload["compose_files"]]
                self.assertEqual(names[0], "docker-compose.prod.yml")
                if mode == "external":
                    self.assertEqual(names[-1], "docker-compose.external-db.yml")
                    self.assertNotIn("forklift-pg-prod", payload["expected_containers"])
                else:
                    self.assertEqual(len(names), 1)
                    self.assertIn("forklift-pg-prod", payload["expected_containers"])

    def test_empty_environment_mode_keeps_saved_external_topology(self):
        with tempfile.TemporaryDirectory() as raw_tmp:
            tmp = Path(raw_tmp)
            environment = self._environment(tmp, "external")
            (tmp / ".env").write_text(
                "DB_MODE=external\n"
                f'DATABASE_URL="{environment["DATABASE_URL"]}"\n',
                encoding="utf-8",
            )
            with patch.dict(os.environ, {
                "DEPLOY_PATH": str(tmp),
                "DB_MODE": "",
                "IMAGE_TAG": "sha-cli",
            }, clear=True):
                deployer = deploy_cli.Deployer("deploy", deploy_cli._parser().parse_args([]))
            self.assertEqual(deployer.database.mode, "external")

    def test_cd_empty_values_do_not_reuse_saved_testing_topology(self):
        with tempfile.TemporaryDirectory() as raw_tmp:
            tmp = Path(raw_tmp)
            (tmp / ".env").write_text(
                "DB_MODE=external\n"
                "POSTGRES_IMAGE=postgres:old\n"
                "PG_VOLUME=pgdata-old\n",
                encoding="utf-8",
            )
            with patch.dict(os.environ, {
                "DEPLOY_PATH": str(tmp),
                "DEPLOY_ENVIRONMENT": "testing",
                "DB_MODE": "",
                "POSTGRES_IMAGE": "",
                "PG_VOLUME": "",
                "IMAGE_TAG": "sha-cli",
            }, clear=True):
                deployer = deploy_cli.Deployer("deploy", deploy_cli._parser().parse_args([]))
            self.assertEqual(deployer.database.mode, "docker")
            self.assertEqual(deployer.postgres_image, "postgres:19.0-trixie")
            self.assertEqual(deployer.pg_volume, "pgdata-testing-19")

    def test_plan_json_includes_topology_conclusion(self):
        with tempfile.TemporaryDirectory() as raw_tmp:
            tmp = Path(raw_tmp)
            with patch.dict(os.environ, self._environment(tmp, "external"), clear=True):
                deployer = deploy_cli.Deployer("deploy", deploy_cli._parser().parse_args([]))
                deployer.plan_data = lambda: []
                deployer.plan_app = lambda: []
                deployer.migration_needed = lambda image: False
                output = io.StringIO()
                with redirect_stdout(output):
                    deployer.print_plan(json_output=True)
            payload = json.loads(output.getvalue())
            self.assertEqual(payload["release_id"], "sha-cli")
            self.assertEqual(payload["topology"]["database_mode"], "external")
            self.assertEqual(payload["topology"]["migration"]["entrypoint"], "compose_run")

    def test_successful_deploy_publishes_snapshot_after_transaction_verify(self):
        with tempfile.TemporaryDirectory() as raw_tmp:
            tmp = Path(raw_tmp)
            with patch.dict(os.environ, self._environment(tmp, "docker"), clear=True):
                deployer = deploy_cli.Deployer("deploy", deploy_cli._parser().parse_args([]))
                deployer.skip_backup = True
                for name in (
                    "preflight",
                    "login_registry",
                    "ensure_registry_proxy",
                    "write_env_file",
                    "_validate_compose",
                    "cleanup_conflicting_containers",
                    "capture_previous_images",
                    "pull_images",
                    "apply_data",
                    "wait_postgres",
                    "cleanup",
                ):
                    setattr(deployer, name, lambda *args, **kwargs: None)
                deployer.plan_data = lambda: []
                deployer.plan_app = lambda: []
                deployer.migration_needed = lambda image: False
                deployer.preflight_migration_check = lambda image: True
                events = []

                class FakeTransaction:
                    def __init__(self, plan, env, on_applied=None):
                        self.applied_services = []

                    def execute(self):
                        events.append("execute")

                    def verify(self):
                        events.append("verify")

                publish = deployer.publish_topology_snapshot

                def publish_after_verify():
                    events.append("publish")
                    publish()

                deployer.publish_topology_snapshot = publish_after_verify
                with patch.object(deploy_cli, "create_release_plan", return_value=object()), \
                        patch.object(deploy_cli, "ReleaseTransaction", FakeTransaction), \
                        patch.object(deploy_cli, "log_ok"):
                    deployer.deploy()

            self.assertEqual(events, ["execute", "verify", "publish"])
            self.assertTrue(snapshot_path_for(tmp).is_file())

    def test_snapshot_check_cli_emits_validated_conclusion_for_consumers(self):
        with tempfile.TemporaryDirectory() as raw_tmp:
            tmp = Path(raw_tmp)
            environment = self._environment(tmp, "external")
            with patch.dict(os.environ, environment, clear=True):
                deployer = deploy_cli.Deployer("deploy", deploy_cli._parser().parse_args([]))
                with patch.object(deploy_cli, "log_ok"):
                    deployer.publish_topology_snapshot()
                output = io.StringIO()
                with patch.dict(os.environ, {
                    "DEPLOY_PATH": str(tmp),
                    "IMAGE_TAG": "sha-cli",
                    "DB_MODE": "external",
                }, clear=True):
                    with redirect_stdout(output):
                        result = deploy_cli.main(["--check-topology-snapshot", "--json"])
            self.assertEqual(result, 0)
            payload = json.loads(output.getvalue())
            self.assertEqual(payload["release_id"], "sha-cli")
            self.assertEqual(payload["backup"]["strategy"], "external_pg_dump")

    def test_snapshot_check_cli_uses_release_marker_without_runtime_identity(self):
        with tempfile.TemporaryDirectory() as raw_tmp:
            tmp = Path(raw_tmp)
            with patch.dict(os.environ, self._environment(tmp, "docker"), clear=True):
                deployer = deploy_cli.Deployer("deploy", deploy_cli._parser().parse_args([]))
                with patch.object(deploy_cli, "log_ok"):
                    deployer.publish_topology_snapshot()
            output = io.StringIO()
            with patch.dict(os.environ, {"DEPLOY_PATH": str(tmp)}, clear=True):
                with redirect_stdout(output):
                    result = deploy_cli.main(["--check-topology-snapshot", "--json"])
            self.assertEqual(result, 0)
            self.assertEqual(json.loads(output.getvalue())["release_id"], "sha-cli")

    def test_snapshot_check_cli_rejects_stale_runtime_identity_and_marker(self):
        with tempfile.TemporaryDirectory() as raw_tmp:
            tmp = Path(raw_tmp)
            with patch.dict(os.environ, self._environment(tmp, "docker"), clear=True):
                deployer = deploy_cli.Deployer("deploy", deploy_cli._parser().parse_args([]))
                with patch.object(deploy_cli, "log_ok"):
                    deployer.publish_topology_snapshot()
            marker = tmp / "backups" / "current-release-id"
            marker.write_text("sha-stale\n", encoding="utf-8")
            with patch.dict(os.environ, {
                "DEPLOY_PATH": str(tmp),
                "IMAGE_TAG": "sha-cli",
            }, clear=True), patch.object(deploy_cli, "log_error"):
                self.assertEqual(deploy_cli.main(["--check-topology-snapshot", "--json"]), 1)

    def test_deployer_publishes_snapshot_at_documented_path(self):
        with tempfile.TemporaryDirectory() as raw_tmp:
            tmp = Path(raw_tmp)
            with patch.dict(os.environ, self._environment(tmp, "external"), clear=True):
                deployer = deploy_cli.Deployer("deploy", deploy_cli._parser().parse_args([]))
                with patch.object(deploy_cli, "log_ok"):
                    deployer.publish_topology_snapshot()
            snapshot = read_topology_snapshot(
                snapshot_path_for(tmp),
                expected_release_id="sha-cli",
            )
            self.assertEqual(snapshot["database_mode"], "external")
            self.assertNotIn("DATABASE_URL", snapshot)


if __name__ == "__main__":
    unittest.main()
