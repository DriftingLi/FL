"""部署拓扑单一 owner 的解析与快照测试。"""
from __future__ import annotations

import json
import os
import sys
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

from effective_config import EffectiveConfig
from topology import (
    TopologySnapshotError,
    read_topology_snapshot,
    resolve_topology,
    snapshot_path_for,
    write_topology_snapshot,
)


class ResolveTopologyTest(unittest.TestCase):
    def test_environment_defaults_preserve_testing_and_production_behavior(self):
        config = EffectiveConfig(declarations={"DB_MODE": "docker", "PG_VOLUME": "pgdata-prod"})
        for environment_name, image, volume in (
            ("testing", "postgres:19.0-trixie", "pgdata-testing-19"),
            ("production", "postgres:18-alpine", "pgdata-prod"),
        ):
            with self.subTest(environment=environment_name):
                topology = resolve_topology(
                    config=config,
                    environment={"DEPLOY_ENVIRONMENT": environment_name},
                    deploy_path=Path("/opt/forklift-training"),
                )
                self.assertEqual(topology.database.mode, "docker")
                self.assertEqual(topology.postgres_image, image)
                self.assertEqual(topology.pg_volume, volume)

    def test_explicit_database_environment_values_are_not_reinterpreted(self):
        config = EffectiveConfig(declarations={"DB_MODE": "docker", "PG_VOLUME": "pgdata-prod"})
        topology = resolve_topology(
            config=config,
            environment={
                "DEPLOY_ENVIRONMENT": "testing",
                "DB_MODE": "external",
                "POSTGRES_IMAGE": "postgres:custom",
                "PG_VOLUME": "/srv/custom-pgdata",
            },
            deploy_path=Path("/opt/forklift-training"),
        )
        self.assertEqual(topology.database.mode, "external")
        self.assertEqual(topology.postgres_image, "postgres:custom")
        self.assertEqual(topology.pg_volume, "/srv/custom-pgdata")

    def test_unknown_deploy_environment_fails_closed(self):
        with self.assertRaises(ValueError):
            resolve_topology(
                config=EffectiveConfig(declarations={"DB_MODE": "docker"}),
                environment={"DEPLOY_ENVIRONMENT": "staging"},
                deploy_path=Path("/opt/forklift-training"),
            )

    def test_docker_and_external_resolve_distinct_real_adapter_policies(self):
        config = EffectiveConfig(declarations={"DB_MODE": "docker"})
        docker = resolve_topology(
            config=config,
            environment={"COMPOSE_PROJECT_NAME": "forklift-training"},
            deploy_path=Path("/opt/forklift-training"),
            profiles=(),
        )
        external = resolve_topology(
            config=config,
            environment={
                "COMPOSE_PROJECT_NAME": "forklift-training",
                "DB_MODE": "external",
            },
            deploy_path=Path("/opt/forklift-training"),
            profiles=(),
        )

        self.assertEqual(docker.database.mode, "docker")
        self.assertEqual(external.database.mode, "external")
        self.assertEqual(
            [path.name for path in docker.database.compose_files],
            ["docker-compose.prod.yml"],
        )
        self.assertEqual(
            [path.name for path in external.database.compose_files],
            ["docker-compose.prod.yml", "docker-compose.external-db.yml"],
        )
        self.assertIn("forklift-pg-prod", docker.database.expected_containers)
        self.assertNotIn("forklift-pg-prod", external.database.expected_containers)
        self.assertEqual(docker.database.data_services, ("postgres",))
        self.assertEqual(external.database.data_services, ())
        self.assertEqual(docker.database.migration.entrypoint, "docker_run")
        self.assertEqual(external.database.migration.entrypoint, "compose_run")
        self.assertEqual(docker.database.backup.strategy, "docker_exec")
        self.assertEqual(external.database.backup.strategy, "external_pg_dump")
        self.assertEqual(docker.database.watchdog.strategy, "docker_postgres")
        self.assertEqual(external.database.watchdog.strategy, "external_postgres_tls")


class TopologySnapshotTest(unittest.TestCase):
    def _topology(self, tmp: Path, **environment):
        merged = {
            "IMAGE_TAG": "sha-current",
            "DATABASE_URL": "postgresql://user:super-secret@db/forklift_training",
            "DB_PASSWORD": "super-secret",
            "SECRET_KEY": "token-value",
            "GITHUB_TOKEN": "ghp-secret",
        }
        merged.update(environment)
        return resolve_topology(
            config=EffectiveConfig(declarations={"DB_MODE": "docker"}),
            environment=merged,
            deploy_path=tmp,
            profiles=(),
        )

    def test_snapshot_is_atomic_permission_safe_and_non_sensitive(self):
        with tempfile.TemporaryDirectory() as raw_tmp:
            tmp = Path(raw_tmp)
            topology = self._topology(tmp)
            path = snapshot_path_for(tmp)
            self.assertEqual(path, tmp / "backups" / "deployment-topology.json")

            write_topology_snapshot(path, topology)
            marker = path.parent / "current-release-id"
            self.assertEqual(marker.read_text(encoding="utf-8"), "sha-current\n")
            first = read_topology_snapshot(path, expected_release_id="sha-current")
            self.assertEqual(first["release_id"], "sha-current")
            self.assertEqual(first["database_mode"], "docker")
            self.assertEqual(first["postgres_image"], "postgres:18-alpine")
            self.assertEqual(first["pg_volume"], "pgdata-prod")
            self.assertIn("forklift-pg-prod", first["expected_containers"])
            if os.name != "nt":
                self.assertEqual(path.stat().st_mode & 0o777, 0o600)
                self.assertEqual(marker.stat().st_mode & 0o777, 0o600)
            self.assertEqual(list(path.parent.glob(f".{path.name}.*.tmp")), [])
            self.assertEqual(list(path.parent.glob(f".{marker.name}.*.tmp")), [])

            raw = path.read_text(encoding="utf-8")
            for secret in ("super-secret", "token-value", "ghp-secret", "DATABASE_URL", "DB_PASSWORD"):
                self.assertNotIn(secret, raw)

            replacement = self._topology(tmp, IMAGE_TAG="sha-next")
            with patch("topology.os.replace", side_effect=OSError("replace failed")):
                with self.assertRaises(OSError):
                    write_topology_snapshot(path, replacement)
            self.assertEqual(
                read_topology_snapshot(path, expected_release_id="sha-current")["release_id"],
                "sha-current",
            )
            self.assertEqual(marker.read_text(encoding="utf-8"), "sha-current\n")
            self.assertEqual(list(path.parent.glob(f".{path.name}.*.tmp")), [])
            self.assertEqual(list(path.parent.glob(f".{marker.name}.*.tmp")), [])

    def test_snapshot_requires_nonempty_postgres_image_and_volume(self):
        with tempfile.TemporaryDirectory() as raw_tmp:
            tmp = Path(raw_tmp)
            path = snapshot_path_for(tmp)
            write_topology_snapshot(path, self._topology(tmp))
            valid = json.loads(path.read_text(encoding="utf-8"))
            for field in ("postgres_image", "pg_volume"):
                for value in (None, "", "   ", 42):
                    with self.subTest(field=field, value=value):
                        damaged = dict(valid)
                        if value is None:
                            damaged.pop(field)
                        else:
                            damaged[field] = value
                        path.write_text(json.dumps(damaged), encoding="utf-8")
                        with self.assertRaises(TopologySnapshotError):
                            read_topology_snapshot(path)

    def test_marker_failure_leaves_snapshot_and_marker_mismatched_fail_closed(self):
        with tempfile.TemporaryDirectory() as raw_tmp:
            tmp = Path(raw_tmp)
            path = snapshot_path_for(tmp)
            marker = path.parent / "current-release-id"
            write_topology_snapshot(path, self._topology(tmp))
            replacement = self._topology(tmp, IMAGE_TAG="sha-next")
            real_replace = os.replace

            def fail_marker_replace(source, destination):
                if Path(destination) == marker:
                    raise OSError("marker replace failed")
                return real_replace(source, destination)

            with patch("topology.os.replace", side_effect=fail_marker_replace):
                with self.assertRaises(OSError):
                    write_topology_snapshot(path, replacement)
            self.assertEqual(marker.read_text(encoding="utf-8"), "sha-current\n")
            with self.assertRaises(TopologySnapshotError):
                read_topology_snapshot(path, expected_release_id="sha-next")
            with self.assertRaises(TopologySnapshotError):
                read_topology_snapshot(path)

    def test_missing_corrupt_and_stale_release_markers_fail_closed(self):
        with tempfile.TemporaryDirectory() as raw_tmp:
            tmp = Path(raw_tmp)
            path = snapshot_path_for(tmp)
            marker = path.parent / "current-release-id"
            write_topology_snapshot(path, self._topology(tmp))

            marker.unlink()
            with self.assertRaises(TopologySnapshotError):
                read_topology_snapshot(path)

            marker.write_text("not a valid release id\n", encoding="utf-8")
            with self.assertRaises(TopologySnapshotError):
                read_topology_snapshot(path)

            marker.write_text("sha-current\nsha-other\n", encoding="utf-8")
            with self.assertRaises(TopologySnapshotError):
                read_topology_snapshot(path)

            marker.write_text("sha-stale\n", encoding="utf-8")
            with self.assertRaises(TopologySnapshotError):
                read_topology_snapshot(path, expected_release_id="sha-current")
            with self.assertRaises(TopologySnapshotError):
                read_topology_snapshot(path)

    def test_snapshot_release_id_prefers_image_tag(self):
        with tempfile.TemporaryDirectory() as raw_tmp:
            topology = self._topology(
                Path(raw_tmp),
                IMAGE_TAG="sha-image",
                GITHUB_SHA="abcdef1234567890",
                RELEASE_ID="release-other",
            )
            self.assertEqual(topology.release_id, "sha-image")

    def test_missing_corrupt_old_incomplete_and_mismatched_snapshots_fail_closed(self):
        with tempfile.TemporaryDirectory() as raw_tmp:
            tmp = Path(raw_tmp)
            path = snapshot_path_for(tmp)
            with self.assertRaises(TopologySnapshotError):
                read_topology_snapshot(path)

            write_topology_snapshot(path, self._topology(tmp))
            valid = json.loads(path.read_text(encoding="utf-8"))

            path.write_text("{not-json", encoding="utf-8")
            with self.assertRaises(TopologySnapshotError):
                read_topology_snapshot(path)

            old = dict(valid, schema_version=0)
            path.write_text(json.dumps(old), encoding="utf-8")
            with self.assertRaises(TopologySnapshotError):
                read_topology_snapshot(path)

            incomplete = dict(valid)
            incomplete.pop("expected_containers")
            path.write_text(json.dumps(incomplete), encoding="utf-8")
            with self.assertRaises(TopologySnapshotError):
                read_topology_snapshot(path)

            incomplete_containers = dict(valid)
            incomplete_containers["expected_containers"] = [
                name for name in valid["expected_containers"]
                if name != "forklift-backend-prod"
            ]
            path.write_text(json.dumps(incomplete_containers), encoding="utf-8")
            with self.assertRaises(TopologySnapshotError):
                read_topology_snapshot(path)

            unsafe_containers = dict(valid)
            unsafe_containers["expected_containers"] = [*valid["expected_containers"], "bad\nname"]
            path.write_text(json.dumps(unsafe_containers), encoding="utf-8")
            with self.assertRaises(TopologySnapshotError):
                read_topology_snapshot(path)

            wrong_migration = dict(valid)
            wrong_migration["migration"] = {
                "entrypoint": "docker_run",
                "command": ["/bin/sh", "-c", "unexpected"],
            }
            path.write_text(json.dumps(wrong_migration), encoding="utf-8")
            with self.assertRaises(TopologySnapshotError):
                read_topology_snapshot(path)

            path.write_bytes(b"\xff\xfe\x00")
            with self.assertRaises(TopologySnapshotError):
                read_topology_snapshot(path)

            path.write_text(json.dumps(valid), encoding="utf-8")
            with self.assertRaises(TopologySnapshotError):
                read_topology_snapshot(path, expected_release_id="sha-stale")


if __name__ == "__main__":
    unittest.main()
