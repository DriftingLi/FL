"""部署拓扑 workflow 与 shell 消费面的公开 seam 测试。"""
from __future__ import annotations

import os
import re
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent.parent
DEPLOY_DIR = ROOT / "deploy"
sys.path.insert(0, str(DEPLOY_DIR))

from effective_config import EffectiveConfig
from topology import resolve_topology, snapshot_path_for, write_topology_snapshot
WORKFLOW_PATH = ROOT / ".github" / "workflows" / "cd.yml"
WATCHDOG_PATH = DEPLOY_DIR / "forklift-watchdog.sh"
BACKUP_PATH = DEPLOY_DIR / "backup-daily.sh"


class WorkflowTopologyTest(unittest.TestCase):
    def setUp(self):
        self.workflow = WORKFLOW_PATH.read_text(encoding="utf-8")

    def test_workflow_passes_environment_and_database_values_without_interpreting_mode(self):
        lines = [line.strip() for line in self.workflow.splitlines()]
        self.assertIn("DEPLOY_ENVIRONMENT: ${{ needs.gate.outputs.environment }}", lines)
        self.assertIn("DB_MODE: ${{ vars.DB_MODE }}", lines)
        self.assertIn("POSTGRES_IMAGE: ${{ vars.POSTGRES_IMAGE }}", lines)
        self.assertIn("PG_VOLUME: ${{ secrets.PG_VOLUME }}", lines)
        for name in ("DB_MODE", "POSTGRES_IMAGE", "PG_VOLUME"):
            value_line = next(line for line in lines if line.startswith(name + ":"))
            self.assertNotRegex(value_line, r"&&|\|\||==|!=")
        self.assertNotRegex(self.workflow, r"if\s+.*DB_MODE|case\s+.*DB_MODE")
        self.assertNotIn("POSTGRES_IMAGE:-postgres:18-alpine", self.workflow)
        self.assertNotIn("PG_VOLUME=pgdata-testing-19", self.workflow)
        self.assertIn("printf 'export DEPLOY_ENVIRONMENT=%q\\n'", self.workflow)

    def test_upload_tar_contains_every_deploy_python_module(self):
        start = self.workflow.index("tar czf /tmp/deploy-files.tar.gz")
        end = self.workflow.index("scp -o ConnectTimeout=15", start)
        tar_block = self.workflow[start:end]
        modules = sorted((ROOT / "deploy").glob("*.py"))
        self.assertGreaterEqual(len(modules), 4)
        for module in modules:
            with self.subTest(module=module.name):
                self.assertIn(f"deploy/{module.name}", tar_block)
class ShellTopologyConsumerTest(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.tmp = Path(self.temporary.name)
        self.bash = shutil.which("bash") or "bash"
        self.remote_deploy = self.tmp / "deploy"
        self.remote_deploy.mkdir()
        for module in DEPLOY_DIR.glob("*.py"):
            shutil.copy2(module, self.remote_deploy / module.name)
        self.bin_dir = self.tmp / "bin"
        self.bin_dir.mkdir()
        self.command_log = self.tmp / "commands.log"
        self.command_log.write_text("", encoding="utf-8")
        self._stub("docker", """#!/usr/bin/env bash
printf 'docker %s\\n' "$*" >> "$COMMAND_LOG"
if [ "$1" = ps ]; then
  while IFS= read -r name; do
    [ -n "$name" ] && printf '%s|Up 2 seconds (healthy)\\n' "$name"
  done <<< "${DOCKER_RUNNING_NAMES:-}"
elif [ "$1" = exec ]; then
  printf 'fake-dump'
fi
""")
        self._stub("pg_dump", """#!/usr/bin/env bash
printf 'pg_dump %s\\n' "$*" >> "$COMMAND_LOG"
printf 'fake-dump'
""")
        self._stub("psql", """#!/usr/bin/env bash
printf 'psql %s\\n' "$*" >> "$COMMAND_LOG"
printf '1\\n'
""")
        self._stub("aws", """#!/usr/bin/env bash
printf 'aws %s\\n' "$*" >> "$COMMAND_LOG"
""")
        self._stub("curl", """#!/usr/bin/env bash
printf 'curl %s\\n' "$*" >> "$COMMAND_LOG"
if [[ "$*" == *--resolve* ]]; then printf '200'; else printf '{"code":200}'; fi
""")
        self._stub("timeout", """#!/usr/bin/env bash
shift
exec "$@"
""")
        self._stub("ssh", """#!/usr/bin/env bash
printf 'ssh %s\\n' "$*" >> "$COMMAND_LOG"
exit 1
""")
        self.watchdog_env = self.tmp / "watchdog.env"
        self.watchdog_env.write_text(
            "WATCH_TO_EMAIL=ops@example.test\n"
            "SMTP_HOST=smtp.example.test\n"
            "SMTP_USERNAME=ops@example.test\n"
            "SMTP_PASSWORD=not-a-real-secret\n"
            "WATCH_PUBLIC_API_URL=https://api.example.test/api/ai-assistant/models\n"
            "WATCH_FE_HOST=api.example.test\n"
            "WATCH_DISK_CRIT_PCT=100\n",
            encoding="utf-8",
        )
        self.deploy_env = self.tmp / "deploy.env"
        self.backup_env = self.tmp / "backup.env"
        self.backup_env.write_text(
            "AWS_ACCESS_KEY_ID=test\n"
            "AWS_SECRET_ACCESS_KEY=test\n"
            "RGW_ENDPOINT=http://rgw.example.test\n"
            "PGSERVICEFILE=/tmp/forklift-backup-service.conf\n"
            "REMOTE_HOST=\n",
            encoding="utf-8",
        )
        self.environment = {
            **os.environ,
            "PATH": str(self.bin_dir) + os.pathsep + os.environ["PATH"],
            "LC_ALL": "C.UTF-8",
            "DEPLOY_PATH": self._bash_path(self.tmp),
            "COMMAND_LOG": self._bash_path(self.command_log),
            "ENV_FILE": self._bash_path(self.watchdog_env),
            "STATE_FILE": self._bash_path(self.tmp / "watchdog-state"),
            "WATCH_SELF_PATH": self._bash_path(WATCHDOG_PATH),
            "WATCH_SCRIPT_REF": self._bash_path(WATCHDOG_PATH),
            "WATCH_DEPLOY_ENV": self._bash_path(self.deploy_env),
            "BACKUP_ENV_FILE": self._bash_path(self.backup_env),
            "BKDIR": self._bash_path(self.tmp / "local-backups"),
            "PG_DUMP_BIN": self._bash_path(self.bin_dir / "pg_dump"),
            "WATCH_PSQL_BIN": self._bash_path(self.bin_dir / "psql"),
        }
        for name in ("IMAGE_TAG", "GITHUB_SHA", "RELEASE_ID", "DB_MODE"):
            self.environment.pop(name, None)

    def _stub(self, name: str, body: str):
        path = self.bin_dir / name
        path.write_text(body, encoding="utf-8", newline="\n")
        path.chmod(0o755)

    def _bash_path(self, path: Path) -> str:
        value = path.as_posix()
        if os.name == "nt":
            return "/" + path.drive[0].lower() + value[2:]
        return value

    def _publish(self, mode: str):
        topology = resolve_topology(
            config=EffectiveConfig(declarations={"DB_MODE": "docker", "PG_VOLUME": "pgdata-prod"}),
            environment={"IMAGE_TAG": "sha-current", "DB_MODE": mode},
            deploy_path=self.tmp,
            profiles=("full",),
        )
        write_topology_snapshot(snapshot_path_for(self.tmp), topology)
        self.deploy_env.write_text(
            f"DB_MODE={'external' if mode == 'docker' else 'docker'}\n",
            encoding="utf-8",
        )
        self.backup_env.write_text(
            self.backup_env.read_text(encoding="utf-8")
            + f"DB_MODE={'external' if mode == 'docker' else 'docker'}\n",
            encoding="utf-8",
        )
        self.environment["DOCKER_RUNNING_NAMES"] = "\n".join(topology.database.expected_containers)
        return topology

    def _run(self, script: Path, *arguments: str):
        completed = subprocess.run(
            [self.bash, "-s", "--", *arguments],
            input=script.read_bytes(),
            cwd=ROOT,
            env=self.environment,
            capture_output=True,
            timeout=30,
        )
        return subprocess.CompletedProcess(
            completed.args,
            completed.returncode,
            completed.stdout.decode("utf-8", errors="replace"),
            completed.stderr.decode("utf-8", errors="replace"),
        )

    def _watchdog(self):
        return self._run(WATCHDOG_PATH, "--dry-run")

    def _backup(self):
        return self._run(BACKUP_PATH)

    def test_shell_consumers_do_not_parse_database_mode_or_assemble_container_list(self):
        for script in (WATCHDOG_PATH, BACKUP_PATH):
            with self.subTest(script=script.name):
                source = script.read_text(encoding="utf-8")
                self.assertNotRegex(source, r"\bDB_MODE\b")
                self.assertIn("--check-topology-snapshot", source)
                self.assertIn("python3", source)
                self.assertNotIn("jq ", source)
        watchdog = WATCHDOG_PATH.read_text(encoding="utf-8")
        self.assertNotIn("EXPECT_CONTAINERS=(forklift-", watchdog)
        self.assertNotIn("forklift-libreoffice-prod", watchdog)

    def test_watchdog_uses_snapshot_containers_and_only_external_tls_probe(self):
        for mode in ("docker", "external"):
            with self.subTest(mode=mode):
                self.command_log.write_text("", encoding="utf-8")
                self._publish(mode)
                result = self._watchdog()
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertNotIn("失败：containers", result.stdout)
                if mode == "docker":
                    self.assertNotIn("psql ", self.command_log.read_text(encoding="utf-8"))
                    self.assertIn("Docker PostgreSQL 由容器巡检覆盖", result.stdout)
                else:
                    self.assertIn("psql ", self.command_log.read_text(encoding="utf-8"))
                    self.assertNotIn("失败：postgres", result.stdout)

    def test_daily_backup_dispatches_dump_strategy_from_snapshot(self):
        for mode in ("docker", "external"):
            with self.subTest(mode=mode):
                self.command_log.write_text("", encoding="utf-8")
                self._publish(mode)
                result = self._backup()
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                commands = self.command_log.read_text(encoding="utf-8")
                if mode == "docker":
                    self.assertIn("docker exec forklift-pg-prod pg_dump", commands)
                    self.assertNotIn("pg_dump -w", commands)
                else:
                    self.assertIn("pg_dump -w", commands)
                    self.assertNotIn("docker exec", commands)
                self.assertIn("aws --endpoint-url", commands)
                self.assertTrue(list((self.tmp / "local-backups").glob("db_*.sql.gz")))

    def test_missing_corrupt_stale_and_mismatched_snapshots_stop_both_consumers(self):
        for script_name in ("watchdog", "backup"):
            for failure in ("missing", "corrupt", "stale_marker", "missing_marker", "mismatch"):
                with self.subTest(script=script_name, failure=failure):
                    self.command_log.write_text("", encoding="utf-8")
                    self._publish("docker")
                    snapshot = snapshot_path_for(self.tmp)
                    marker = snapshot.parent / "current-release-id"
                    if failure == "missing":
                        snapshot.unlink()
                        marker.unlink()
                    elif failure == "corrupt":
                        snapshot.write_text("{not-json", encoding="utf-8")
                    elif failure == "stale_marker":
                        marker.write_text("sha-stale\n", encoding="utf-8")
                    elif failure == "missing_marker":
                        marker.unlink()
                    else:
                        self.environment["IMAGE_TAG"] = "sha-next"
                    result = self._watchdog() if script_name == "watchdog" else self._backup()
                    self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
                    commands = self.command_log.read_text(encoding="utf-8")
                    self.assertNotIn("docker ", commands)
                    self.assertNotIn("pg_dump ", commands)
                    self.assertNotIn("aws ", commands)
                    self.environment.pop("IMAGE_TAG", None)


if __name__ == "__main__":
    unittest.main()
