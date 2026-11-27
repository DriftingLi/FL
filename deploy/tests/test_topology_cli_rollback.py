"""部署拓扑在回滚与跨拓扑切换链路中的测试。"""
from __future__ import annotations

import importlib.util
import os
import sys
import tempfile
import unittest
from pathlib import Path
from unittest.mock import Mock, patch

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

from release_transaction import (
    ReleasePlan,
    ServiceChange,
    load_release_plan,
    release_plan_path_for,
)
from topology import read_topology_snapshot, snapshot_path_for

DEPLOY_MODULE_PATH = Path(__file__).resolve().parent.parent / "deploy.py"
_SPEC = importlib.util.spec_from_file_location("deploy_cli_topology_rollback_test", DEPLOY_MODULE_PATH)
assert _SPEC and _SPEC.loader
deploy_cli = importlib.util.module_from_spec(_SPEC)
sys.modules[_SPEC.name] = deploy_cli
_SPEC.loader.exec_module(deploy_cli)


class DeployCliTopologyRollbackTest(unittest.TestCase):
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

    def _external_rollback_plan(self, deployer, tmp: Path):
        environment = {
            **self._environment(tmp, "external"),
            "POSTGRES_IMAGE": "postgres:previous",
            "PG_VOLUME": "/srv/pgdata-previous",
        }
        with patch.dict(os.environ, environment, clear=True):
            previous_deployer = deploy_cli.Deployer("deploy", deploy_cli._parser().parse_args([]))
        return ReleasePlan(
            services=(ServiceChange("backend", "b:old", "b:new", "recreate", "test"),),
            release_id="sha-target",
            previous_release_id="sha-old",
            previous_topology={**previous_deployer.topology.to_dict(), "release_id": "sha-old"},
            target_topology={**deployer.topology.to_dict(), "release_id": "sha-target"},
            migration={
                "needed": False, "preflight_verified": True,
                "previous_backend_image": "b:old", "target_backend_image": "b:new",
            },
            persisted_applied_services=("backend",),
        )

    def test_manual_rollback_prepares_previous_data_services_before_transaction(self):
        for current_mode, previous_mode in (("external", "docker"), ("docker", "external")):
            with self.subTest(current=current_mode, previous=previous_mode), tempfile.TemporaryDirectory() as raw_tmp:
                tmp = Path(raw_tmp)
                current_environment = {
                    **self._environment(tmp, current_mode),
                    "POSTGRES_IMAGE": "postgres:current",
                    "PG_VOLUME": "pgdata-current",
                }
                previous_environment = {
                    **self._environment(tmp, previous_mode),
                    "POSTGRES_IMAGE": "postgres:previous",
                    "PG_VOLUME": "/srv/pgdata-previous",
                }
                with patch.dict(os.environ, current_environment, clear=True):
                    deployer = deploy_cli.Deployer("deploy", deploy_cli._parser().parse_args([]))
                with patch.dict(os.environ, previous_environment, clear=True):
                    previous_deployer = deploy_cli.Deployer("deploy", deploy_cli._parser().parse_args([]))
                previous_topology = {**previous_deployer.topology.to_dict(), "release_id": "sha-old"}
                target_topology = {**deployer.topology.to_dict(), "release_id": "sha-target"}
                if previous_mode == "external":
                    dsn = self._environment(tmp, "external")["DATABASE_URL"]
                    (tmp / ".env").write_text(
                        f'DB_MODE=docker\nDATABASE_URL="{dsn}"\n', encoding="utf-8",
                    )
                plan = ReleasePlan(
                    services=(ServiceChange("backend", "b:old", "b:new", "recreate", "test"),),
                    release_id="sha-target",
                    previous_release_id="sha-old",
                    previous_topology=previous_topology,
                    target_topology=target_topology,
                    migration={
                        "needed": False, "preflight_verified": True,
                        "previous_backend_image": "b:old", "target_backend_image": "b:new",
                    },
                    persisted_applied_services=("backend",),
                )
                events = []

                class FakeTransaction:
                    def __init__(self, restored, env):
                        self.plan = restored
                        events.append("restore_transaction")

                    def rollback(self):
                        events.append(("rollback", deployer.topology.release_id, deployer.database.mode))

                def plan_data():
                    events.append(("plan_data", deployer.database.mode, tuple(deployer.database.data_services)))
                    return []

                def apply_data(decisions):
                    events.append((
                        "apply_data", deployer.database.mode, tuple(deployer.database.data_services),
                        tuple(path.name for path in deployer.topology.database.compose_files),
                    ))

                deployer.plan_data = plan_data
                deployer.apply_data = apply_data
                deployer.wait_postgres = lambda: events.append(("wait_postgres", deployer.database.mode))
                with patch.object(deploy_cli, "load_release_plan", return_value=plan), \
                        patch.object(deploy_cli.ReleaseTransaction, "restore", FakeTransaction), \
                        patch.object(deploy_cli, "log_ok"), patch.object(deploy_cli, "log_warn"), \
                        patch.object(deploy_cli, "log_info"):
                    deployer.do_rollback()
                self.assertEqual(events, [
                    ("plan_data", previous_mode, tuple(previous_deployer.database.data_services)),
                    ("apply_data", previous_mode, tuple(previous_deployer.database.data_services),
                     tuple(path.name for path in previous_deployer.topology.database.compose_files)),
                    ("wait_postgres", previous_mode),
                    "restore_transaction",
                    ("rollback", "sha-old", previous_mode),
                ])
                snapshot = read_topology_snapshot(snapshot_path_for(tmp))
                self.assertEqual(snapshot["release_id"], "sha-old")
                self.assertEqual(snapshot["expected_containers"], previous_topology["expected_containers"])
                self.assertEqual(snapshot["postgres_image"], previous_topology["postgres_image"])
                self.assertEqual(snapshot["pg_volume"], previous_topology["pg_volume"])
                self.assertEqual(deployer.topology.postgres_image, previous_topology["postgres_image"])
                self.assertEqual(deployer.topology.pg_volume, previous_topology["pg_volume"])
                self.assertEqual(deployer.postgres_image, previous_topology["postgres_image"])
                self.assertEqual(deployer.pg_volume, previous_topology["pg_volume"])
                if previous_mode == "external":
                    self.assertEqual(deployer.external_db_url, previous_deployer.external_db_url)
                    self.assertEqual(deployer.external_pg_settings, previous_deployer.external_pg_settings)
                else:
                    self.assertEqual(deployer.external_db_url, "")
                    self.assertEqual(deployer.external_pg_settings, {})
                self.assertEqual(load_release_plan(release_plan_path_for(tmp)).release_id, "sha-old")

    def test_rollback_to_external_configures_environment_dsn_before_data_services(self):
        with tempfile.TemporaryDirectory() as raw_tmp:
            tmp = Path(raw_tmp)
            with patch.dict(os.environ, self._environment(tmp, "docker"), clear=True):
                deployer = deploy_cli.Deployer("rollback", deploy_cli._parser().parse_args([]))
            plan = self._external_rollback_plan(deployer, tmp)
            dsn = self._environment(tmp, "external")["DATABASE_URL"]
            events = []

            def plan_data():
                events.append("plan_data")
                self.assertEqual(deployer.external_db_url, dsn)
                self.assertEqual(deployer.external_pg_settings["PGHOST"], "pg.internal")
                self.assertEqual(deployer.external_pg_settings["PGPASSWORD"], "local&test")
                self.assertEqual(deployer.database.data_services, ())
                return []

            deployer.plan_data = plan_data
            deployer.apply_data = lambda decisions: events.append("apply_data")
            deployer.wait_postgres = lambda: events.append("wait_postgres")
            with patch.dict(os.environ, {"DATABASE_URL": dsn}, clear=True), \
                    patch.object(deploy_cli, "load_release_plan", return_value=plan), \
                    patch.object(deploy_cli.ReleaseTransaction, "restore", return_value=Mock(plan=plan)), \
                    patch.object(deploy_cli, "log_ok"), patch.object(deploy_cli, "log_warn"), \
                    patch.object(deploy_cli, "log_info"):
                deployer.do_rollback()
            self.assertEqual(events, ["plan_data", "apply_data", "wait_postgres"])
            self.assertEqual(deployer.postgres_image, "postgres:previous")
            self.assertEqual(deployer.pg_volume, "/srv/pgdata-previous")

    def test_rollback_to_external_rejects_missing_or_invalid_dsn_before_data_services(self):
        for saved_dsn in (None, "postgresql://forklift:secret@pg.internal/wrong_database"):
            with self.subTest(saved_dsn=saved_dsn), tempfile.TemporaryDirectory() as raw_tmp:
                tmp = Path(raw_tmp)
                with patch.dict(os.environ, self._environment(tmp, "docker"), clear=True):
                    deployer = deploy_cli.Deployer("rollback", deploy_cli._parser().parse_args([]))
                if saved_dsn is not None:
                    (tmp / ".env").write_text(
                        f'DB_MODE=docker\nDATABASE_URL="{saved_dsn}"\n', encoding="utf-8",
                    )
                plan = self._external_rollback_plan(deployer, tmp)
                events = []
                deployer.plan_data = lambda: events.append("plan_data")
                deployer.apply_data = lambda decisions: events.append("apply_data")
                deployer.wait_postgres = lambda: events.append("wait_postgres")
                with patch.dict(os.environ, {}, clear=True), \
                        patch.object(deploy_cli, "load_release_plan", return_value=plan), \
                        patch.object(deploy_cli.ReleaseTransaction, "restore") as restore, \
                        patch.object(deploy_cli, "log_ok"), patch.object(deploy_cli, "log_warn"), \
                        patch.object(deploy_cli, "log_info"):
                    with self.assertRaises(deploy_cli.DeployError):
                        deployer.do_rollback()
                self.assertEqual(events, [])
                restore.assert_not_called()
                self.assertEqual(deployer.external_db_url, "")
                self.assertEqual(deployer.external_pg_settings, {})
                self.assertEqual(deployer.database.mode, "docker")

    def test_manual_rollback_rejects_damaged_previous_topology_before_data_services(self):
        damaged_topologies = (
            None,
            {"release_id": "sha-old", "database_mode": "broken"},
            {"release_id": "wrong", "database_mode": "docker", "compose_files": []},
        )
        for previous_topology in damaged_topologies:
            with self.subTest(previous_topology=previous_topology), tempfile.TemporaryDirectory() as raw_tmp:
                tmp = Path(raw_tmp)
                with patch.dict(os.environ, self._environment(tmp, "external"), clear=True):
                    deployer = deploy_cli.Deployer("deploy", deploy_cli._parser().parse_args([]))
                plan = ReleasePlan(
                    services=(ServiceChange("backend", "b:old", "b:new", "recreate", "test"),),
                    release_id="sha-target",
                    previous_release_id="sha-old",
                    previous_topology=previous_topology,
                    target_topology=deployer.topology.to_dict(),
                    migration={
                        "needed": False, "preflight_verified": True,
                        "previous_backend_image": "b:old", "target_backend_image": "b:new",
                    },
                    persisted_applied_services=("backend",),
                )
                events = []
                deployer.plan_data = lambda: events.append("plan_data")
                deployer.apply_data = lambda decisions: events.append("apply_data")
                deployer.wait_postgres = lambda: events.append("wait_postgres")
                with patch.object(deploy_cli, "load_release_plan", return_value=plan), \
                        patch.object(deploy_cli.ReleaseTransaction, "restore") as restore, \
                        patch.object(deploy_cli, "log_ok"), patch.object(deploy_cli, "log_warn"), \
                        patch.object(deploy_cli, "log_info"):
                    with self.assertRaises(deploy_cli.DeployError):
                        deployer.do_rollback()
                self.assertEqual(events, [])
                restore.assert_not_called()


if __name__ == "__main__":
    unittest.main()
