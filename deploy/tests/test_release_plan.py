"""版本化发布计划与共享回滚生命周期的公开 seam 测试。"""
from __future__ import annotations

import json
import sys
import tempfile
import unittest
from dataclasses import replace
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

from release_transaction import (
    ReleasePlanError,
    ReleaseTransaction,
    create_release_plan,
    load_release_plan,
    persist_applied_services,
    release_plan_path_for,
    rollback_plan,
    save_release_plan,
)
from effective_config import EffectiveConfig
from topology import (
    read_topology_snapshot,
    resolve_topology,
    snapshot_path_for,
    write_topology_snapshot,
)
from test_release_transaction import FakeEnvironment


def topology_payload(release_id: str) -> dict:
    return {
        "schema_version": 1,
        "release_id": release_id,
        "database_mode": "docker",
        "postgres_image": "postgres:18-alpine",
        "pg_volume": "pgdata-prod",
        "compose_files": ["/opt/forklift/deploy/docker-compose.prod.yml"],
        "expected_containers": [
            "forklift-backend-prod", "forklift-frontend-prod",
            "forklift-open-school-prod", "forklift-pg-prod",
        ],
        "migration": {"entrypoint": "docker_run", "command": ["/app/bin/migrate", "up"]},
        "backup": {"strategy": "docker_exec"},
        "watchdog": {"strategy": "docker_postgres"},
    }


class FakeTopology:
    def __init__(self, release_id: str, mode: str = "docker"):
        self.release_id = release_id
        self.mode = mode

    def to_dict(self):
        payload = topology_payload(self.release_id)
        if self.mode == "external":
            payload["database_mode"] = "external"
            payload["compose_files"] = [
                "/opt/forklift/deploy/docker-compose.prod.yml",
                "/opt/forklift/deploy/docker-compose.external-db.yml",
            ]
            payload["expected_containers"].remove("forklift-pg-prod")
            payload["migration"]["entrypoint"] = "compose_run"
            payload["backup"]["strategy"] = "external_pg_dump"
            payload["watchdog"]["strategy"] = "external_postgres_tls"
        return payload


class VersionedReleasePlanTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.path = release_plan_path_for(self.root)
        self.env = FakeEnvironment({
            "backend": "registry:5000/org/backend:sha-old",
            "frontend": "registry:5000/org/frontend:sha-old",
            "open_school": "registry:5000/org/backend:sha-old",
        })

    def create(self, release_id="sha-new"):
        return create_release_plan(
            env=self.env,
            desired_images={
                "backend": "registry:5000/org/backend:sha-new",
                "frontend": "registry:5000/org/frontend:sha-new",
                "open_school": "registry:5000/org/backend:sha-new",
            },
            decisions=None,
            migration_needed=True,
            migration_preflight_verified=True,
            release_id=release_id,
            topology=FakeTopology(release_id),
            path=self.path,
            last_version_path=self.root / "backups" / "last-version.txt",
            fallback_previous_topology=FakeTopology("sha-old").to_dict(),
        )

    def test_plan_round_trip_contains_release_images_migration_and_topology(self):
        plan = self.create()
        restored = load_release_plan(self.path)
        self.assertEqual(restored, plan)
        self.assertEqual(restored.release_id, "sha-new")
        self.assertEqual(restored.previous_release_id, "sha-old")
        self.assertEqual(restored.previous_topology["release_id"], "sha-old")
        self.assertEqual(restored.target_topology["release_id"], "sha-new")
        self.assertEqual(restored.migration["needed"], True)
        self.assertEqual(restored.migration["preflight_verified"], True)
        raw = self.path.read_text(encoding="utf-8")
        self.assertNotIn("DATABASE_URL", raw)
        self.assertNotIn("PASSWORD", raw)

    def test_automatic_and_manual_rollback_restore_same_persisted_transaction(self):
        plan = self.create()
        automatic = ReleaseTransaction(plan, self.env, on_applied=lambda applied: persist_applied_services(
            self.path, plan, applied,
        ))
        automatic.execute()
        manual = ReleaseTransaction.restore(load_release_plan(self.path), self.env)
        self.assertEqual(manual.plan, load_release_plan(self.path))
        self.assertEqual(manual.plan.services, automatic.plan.services)
        self.assertEqual(manual.applied_services, automatic.applied_services)

    def test_missing_corrupt_and_incomplete_plans_fail_closed(self):
        with self.assertRaises(ReleasePlanError):
            load_release_plan(self.path)
        self.create()
        valid = json.loads(self.path.read_text(encoding="utf-8"))
        for damaged in (
            {**valid, "schema_version": 99},
            {key: value for key, value in valid.items() if key != "previous_release_id"},
            {**valid, "services": [{**valid["services"][0], "previous_image": ""}]},
            {**valid, "migration": {"needed": True}},
            {**valid, "release_id": 42},
            {**valid, "services": [{**valid["services"][0], "reason": 42}]},
            {**valid, "target_topology": {**valid["target_topology"], "release_id": "other"}},
        ):
            with self.subTest(damaged=damaged):
                self.path.write_text(json.dumps(damaged), encoding="utf-8")
                with self.assertRaises(ReleasePlanError):
                    load_release_plan(self.path)
        self.path.write_text("{broken", encoding="utf-8")
        with self.assertRaises(ReleasePlanError):
            load_release_plan(self.path)

    def test_next_release_uses_previous_plan_topology_not_current_desired_topology(self):
        first_topology = resolve_topology(
            config=EffectiveConfig(declarations={"DB_MODE": "docker"}),
            environment={"IMAGE_TAG": "v1"}, deploy_path=self.root,
        )
        create_release_plan(
            env=self.env,
            desired_images={"backend": "b:v1", "frontend": "f:v1", "open_school": "b:v1"},
            decisions=None,
            migration_needed=False,
            migration_preflight_verified=True,
            release_id="v1",
            topology=first_topology,
            path=self.path,
            last_version_path=self.root / "backups" / "last-version.txt",
            fallback_previous_topology=first_topology.to_dict(),
        )
        write_topology_snapshot(snapshot_path_for(self.root), first_topology)
        target_topology = resolve_topology(
            config=EffectiveConfig(declarations={"DB_MODE": "external"}),
            environment={"IMAGE_TAG": "v2", "DB_MODE": "external"}, deploy_path=self.root,
        )
        plan = create_release_plan(
            env=self.env,
            desired_images={"backend": "b:v2", "frontend": "f:v2", "open_school": "b:v2"},
            decisions=None,
            migration_needed=False,
            migration_preflight_verified=True,
            release_id="v2",
            topology=target_topology,
            path=self.path,
            last_version_path=self.root / "backups" / "last-version.txt",
        )
        self.assertEqual(plan.previous_topology["database_mode"], "docker")
        self.assertEqual(plan.target_topology["database_mode"], "external")
        self.assertEqual(plan.previous_release_id, "v1")

    def test_unresolvable_previous_release_id_fails_closed(self):
        with self.assertRaises(ReleasePlanError):
            create_release_plan(
                env=self.env,
                desired_images={"backend": "b:new", "frontend": "f:new"},
                decisions=None,
                migration_needed=False,
                migration_preflight_verified=True,
                release_id="new",
                topology=FakeTopology("new"),
                path=self.path,
                last_version_path=self.root / "backups" / "last-version.txt",
                previous_images={
                    "backend": "registry:5000/org/backend:old-a",
                    "frontend": "registry:5000/org/frontend:old-b",
                },
                fallback_previous_topology=FakeTopology("old-a").to_dict(),
            )

    def test_saved_environment_supplies_previous_topology_without_previous_plan(self):
        plan = create_release_plan(
            env=self.env,
            desired_images={"backend": "b:new", "frontend": "f:new", "open_school": "b:new"},
            decisions=None,
            migration_needed=False,
            migration_preflight_verified=True,
            release_id="new",
            topology=FakeTopology("new", mode="external"),
            path=self.path,
            last_version_path=self.root / "backups" / "last-version.txt",
            fallback_previous_topology=FakeTopology("placeholder", mode="docker").to_dict(),
        )
        self.assertEqual(plan.previous_release_id, "sha-old")
        self.assertEqual(plan.previous_topology["release_id"], "sha-old")
        self.assertEqual(plan.previous_topology["database_mode"], "docker")
        self.assertEqual(plan.target_topology["database_mode"], "external")

    def test_historical_images_without_saved_environment_fail_closed(self):
        self.root.joinpath("backups").mkdir()
        self.root.joinpath("backups", "last-version.txt").write_text(
            "PREVIOUS_BACKEND_IMAGE=b:old\nPREVIOUS_FRONTEND_IMAGE=f:old\n",
            encoding="utf-8",
        )
        with self.assertRaisesRegex(ReleasePlanError, "previous|previous_topology|保存|历史"):
            create_release_plan(
                env=self.env,
                desired_images={"backend": "b:new", "frontend": "f:new"},
                decisions=None,
                migration_needed=False,
                migration_preflight_verified=True,
                release_id="new",
                topology=FakeTopology("new", mode="external"),
                path=self.path,
                last_version_path=self.root / "backups" / "last-version.txt",
            )

    def test_first_deployment_has_no_previous_release_or_topology(self):
        plan = create_release_plan(
            env=FakeEnvironment(),
            desired_images={"backend": "b:new", "frontend": "f:new"},
            decisions=None,
            migration_needed=False,
            migration_preflight_verified=True,
            release_id="new",
            topology=FakeTopology("new"),
            path=self.path,
            last_version_path=self.root / "backups" / "last-version.txt",
        )
        self.assertIsNone(plan.previous_release_id)
        self.assertIsNone(plan.previous_topology)
        self.assertEqual(load_release_plan(self.path), plan)

    def test_stale_plan_with_no_running_containers_becomes_first_release(self):
        self.create()
        write_topology_snapshot(snapshot_path_for(self.root), FakeTopology("sha-new"))
        plan = create_release_plan(
            env=FakeEnvironment(),
            desired_images={
                "backend": "registry:5000/org/backend:sha-next",
                "frontend": "registry:5000/org/frontend:sha-next",
                "open_school": "registry:5000/org/backend:sha-next",
            },
            decisions=None,
            migration_needed=False,
            migration_preflight_verified=True,
            release_id="sha-next",
            topology=FakeTopology("sha-next"),
            path=self.path,
            last_version_path=self.root / "backups" / "last-version.txt",
        )
        self.assertIsNone(plan.previous_release_id)
        self.assertIsNone(plan.previous_topology)
        self.assertTrue(all(change.previous_image == "unknown" for change in plan.services))
        self.assertEqual(load_release_plan(self.path), plan)

    def test_stale_plan_with_any_running_container_restores_previous_release(self):
        self.create()
        write_topology_snapshot(snapshot_path_for(self.root), FakeTopology("sha-new"))
        plan = create_release_plan(
            env=FakeEnvironment({
                "backend": "registry:5000/org/backend:sha-new",
            }),
            desired_images={
                "backend": "registry:5000/org/backend:sha-next",
                "frontend": "registry:5000/org/frontend:sha-next",
                "open_school": "registry:5000/org/backend:sha-next",
            },
            decisions=None,
            migration_needed=False,
            migration_preflight_verified=True,
            release_id="sha-next",
            topology=FakeTopology("sha-next"),
            path=self.path,
            last_version_path=self.root / "backups" / "last-version.txt",
        )
        self.assertEqual(plan.previous_release_id, "sha-new")
        self.assertEqual(plan.previous_topology["release_id"], "sha-new")
        previous_images = {change.service: change.previous_image for change in plan.services}
        self.assertEqual(previous_images["backend"], "registry:5000/org/backend:sha-new")
        self.assertEqual(previous_images["frontend"], "unknown")
        self.assertEqual(previous_images["open_school"], "unknown")

    def test_running_images_without_previous_plan_or_fallback_fail_closed(self):
        with self.assertRaisesRegex(ReleasePlanError, "previous_topology|保存环境|历史镜像"):
            create_release_plan(
                env=FakeEnvironment({"backend": "registry:5000/org/backend:sha-old"}),
                desired_images={"backend": "registry:5000/org/backend:sha-new"},
                decisions=None,
                migration_needed=False,
                migration_preflight_verified=True,
                release_id="sha-new",
                topology=FakeTopology("sha-new"),
                path=self.path,
                last_version_path=self.root / "backups" / "last-version.txt",
            )

    def test_first_deployment_plan_rejects_invented_previous_state(self):
        plan = create_release_plan(
            env=FakeEnvironment(),
            desired_images={"backend": "b:new"},
            decisions=None,
            migration_needed=False,
            migration_preflight_verified=True,
            release_id="new",
            topology=FakeTopology("new"),
            path=self.path,
            last_version_path=self.root / "backups" / "last-version.txt",
        )
        for invalid in (
            replace(plan, previous_release_id="old"),
            replace(plan, previous_topology=FakeTopology("old").to_dict()),
        ):
            with self.subTest(invalid=invalid):
                with self.assertRaises(ReleasePlanError):
                    save_release_plan(self.path, invalid)


class SharedRollbackTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.path = release_plan_path_for(self.root)
        self.snapshot = snapshot_path_for(self.root)
        self.env = FakeEnvironment({
            "backend": "b:v1", "frontend": "f:v1", "open_school": "b:v1",
        })

    def create(self):
        return create_release_plan(
            env=self.env,
            desired_images={"backend": "b:v2", "frontend": "f:v2", "open_school": "b:v2"},
            decisions=None,
            migration_needed=True,
            migration_preflight_verified=True,
            release_id="v2",
            topology=FakeTopology("v2"),
            path=self.path,
            last_version_path=self.root / "backups" / "last-version.txt",
            fallback_previous_topology=FakeTopology("v1").to_dict(),
        )

    def test_partial_failure_continues_in_reverse_order_verifies_and_aggregates(self):
        plan = self.create()
        plan = persist_applied_services(self.path, plan, [c.service for c in plan.applied_services])
        txn = ReleaseTransaction.restore(plan, self.env)
        real_recreate = self.env.recreate_service

        def fail_frontend(service, force=False):
            if service == "frontend":
                self.env.calls.append(("recreate_service", service))
                raise RuntimeError("frontend recreate failed")
            real_recreate(service, force)

        self.env.recreate_service = fail_frontend
        with self.assertRaises(RuntimeError) as caught:
            txn.rollback()
        message = str(caught.exception)
        self.assertIn("frontend", message)
        recreated = [call[1] for call in self.env.calls if call[0] == "recreate_service"]
        self.assertEqual(recreated, ["open_school", "frontend", "backend"])
        verified = [call[1] for call in self.env.calls if call[0] == "verify_running_image"]
        self.assertEqual(verified, ["open_school", "frontend", "backend"])
        self.assertIn(("health_check",), self.env.calls)

    def test_backend_preflight_failure_still_rolls_back_other_services_and_fails_overall(self):
        plan = self.create()
        plan = persist_applied_services(self.path, plan, [c.service for c in plan.applied_services])
        self.env.migration_preflight_ok = False
        with self.assertRaises(RuntimeError) as caught:
            ReleaseTransaction.restore(plan, self.env).rollback()
        self.assertIn("拒绝回滚后端", str(caught.exception))
        recreated = [call[1] for call in self.env.calls if call[0] == "recreate_service"]
        self.assertEqual(recreated, ["open_school", "frontend"])
        self.assertIn(("verify_running_image", "backend", "b:v1"), self.env.calls)
        self.assertIn(("health_check",), self.env.calls)

    def test_rollback_plan_restores_previous_release_images_and_topology(self):
        plan = self.create()
        plan = persist_applied_services(self.path, plan, [c.service for c in plan.applied_services])
        txn = ReleaseTransaction.restore(plan, self.env)
        txn.rollback()
        write_topology_snapshot(self.snapshot, resolve_topology(
            config=EffectiveConfig(declarations={"DB_MODE": "docker"}),
            environment={"IMAGE_TAG": "v2"}, deploy_path=self.root,
        ))
        previous = rollback_plan(plan, self.path, self.root / "backups" / "last-version.txt")
        self.assertEqual(previous.release_id, "v1")
        self.assertEqual(previous.previous_release_id, "v2")
        self.assertEqual(previous.target_topology["release_id"], "v1")
        self.assertEqual(previous.services[0].target_image, "b:v1")
        restored = load_release_plan(self.path)
        self.assertEqual(restored.release_id, "v1")
        self.assertEqual(restored.target_topology["release_id"], "v1")


if __name__ == "__main__":
    unittest.main()
