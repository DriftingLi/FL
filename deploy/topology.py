"""部署拓扑单一 owner：配置取值 → compose / 容器 / 迁移 / 备份 / 巡检策略。

adapter 只解析策略，不执行 Docker；快照函数只负责版本化序列化与原子发布。
EffectiveConfig 仍只负责取值优先级；ReleaseTransaction 仍只负责应用发布生命周期。
"""
from __future__ import annotations

import json
import os
import re
import tempfile
from dataclasses import dataclass
from pathlib import Path
from typing import Dict, Mapping, Optional, Protocol, Sequence, Tuple

from effective_config import EffectiveConfig

SCHEMA_VERSION = 1
BASE_COMPOSE_FILE = "docker-compose.prod.yml"
EXTERNAL_COMPOSE_FILE = "docker-compose.external-db.yml"
SNAPSHOT_RELATIVE_PATH = Path("backups") / "deployment-topology.json"
CURRENT_RELEASE_ID_RELATIVE_PATH = Path("backups") / "current-release-id"

_APP_CONTAINERS = (
    "forklift-backend-prod",
    "forklift-frontend-prod",
    "forklift-open-school-prod",
)
_PROFILE_CONTAINERS = {
    "full": ("forklift-libreoffice-prod",),
    "collector": ("forklift-collector-prod",),
}
_SAFE_RELEASE_ID = re.compile(r"[A-Za-z0-9][A-Za-z0-9._-]{0,127}\Z")
_CONTAINER_NAME = re.compile(r"[A-Za-z0-9][A-Za-z0-9_.-]{0,127}\Z")
_GIT_SHA = re.compile(r"[0-9a-fA-F]{7,40}\Z")


@dataclass(frozen=True)
class MigrationPolicy:
    entrypoint: str
    command: Tuple[str, ...]
    dirty_state_strategy: str
    require_image_metadata: bool


@dataclass(frozen=True)
class BackupPolicy:
    strategy: str
    synchronous: bool


@dataclass(frozen=True)
class WatchdogPolicy:
    strategy: str


class DatabaseAdapter(Protocol):
    """docker / external 两种真实 adapter 共用的拓扑接口。"""

    mode: str
    compose_files: Tuple[Path, ...]
    expected_containers: Tuple[str, ...]
    data_services: Tuple[str, ...]
    base_image_services: Tuple[str, ...]
    migration: MigrationPolicy
    backup: BackupPolicy
    watchdog: WatchdogPolicy
    readiness_strategy: str
    supports_full_down: bool


class DockerDatabaseAdapter:
    __slots__ = ("compose_files", "expected_containers")

    mode = "docker"
    data_services = ("postgres",)
    base_image_services = ("postgres",)
    migration = MigrationPolicy(
        entrypoint="docker_run",
        command=("/app/bin/migrate", "up"),
        dirty_state_strategy="auto_fix",
        require_image_metadata=False,
    )
    backup = BackupPolicy(strategy="docker_exec", synchronous=False)
    watchdog = WatchdogPolicy(strategy="docker_postgres")
    readiness_strategy = "docker_exec"
    supports_full_down = True

    def __init__(self, compose_files: Tuple[Path, ...], expected_containers: Tuple[str, ...]):
        self.compose_files = tuple(compose_files)
        self.expected_containers = tuple(expected_containers)


class ExternalDatabaseAdapter:
    __slots__ = ("compose_files", "expected_containers")

    mode = "external"
    data_services = ()
    base_image_services = ()
    migration = MigrationPolicy(
        entrypoint="compose_run",
        command=("/app/bin/migrate", "up"),
        dirty_state_strategy="reject",
        require_image_metadata=True,
    )
    backup = BackupPolicy(strategy="external_pg_dump", synchronous=True)
    watchdog = WatchdogPolicy(strategy="external_postgres_tls")
    readiness_strategy = "external_query"
    supports_full_down = False

    def __init__(self, compose_files: Tuple[Path, ...], expected_containers: Tuple[str, ...]):
        self.compose_files = tuple(compose_files)
        self.expected_containers = tuple(expected_containers)


@dataclass(frozen=True)
class DeploymentTopology:
    schema_version: int
    release_id: str
    database: DatabaseAdapter
    postgres_image: str
    pg_volume: str

    def to_dict(self) -> Dict[str, object]:
        return {
            "schema_version": self.schema_version,
            "release_id": self.release_id,
            "database_mode": self.database.mode,
            "postgres_image": self.postgres_image,
            "pg_volume": self.pg_volume,
            "compose_files": [str(path) for path in self.database.compose_files],
            "expected_containers": list(self.database.expected_containers),
            "migration": {
                "entrypoint": self.database.migration.entrypoint,
                "command": list(self.database.migration.command),
            },
            "backup": {"strategy": self.database.backup.strategy},
            "watchdog": {"strategy": self.database.watchdog.strategy},
        }


def resolve_release_id(environment: Mapping[str, str]) -> str:
    """发布标识优先 IMAGE_TAG，其次使用可验证的 SHA / 显式 RELEASE_ID。"""
    image_tag = (environment.get("IMAGE_TAG") or "").strip()
    if image_tag and image_tag.lower() not in ("latest", "unknown") and _SAFE_RELEASE_ID.fullmatch(image_tag):
        return image_tag

    github_sha = (environment.get("GITHUB_SHA") or "").strip()
    if _GIT_SHA.fullmatch(github_sha):
        return github_sha

    release_id = (environment.get("RELEASE_ID") or "").strip()
    if _SAFE_RELEASE_ID.fullmatch(release_id):
        return release_id
    return ""


def _resolved_path(raw: str, deploy_path: Path) -> Path:
    path = Path(raw)
    return (path if path.is_absolute() else deploy_path / path).resolve()


def _base_compose_file(resolved: Mapping[str, str], deploy_path: Path) -> Path:
    raw = (resolved.get("COMPOSE_FILE") or "").strip()
    composed = _resolved_path(raw, deploy_path) if raw else (deploy_path / "deploy" / BASE_COMPOSE_FILE).resolve()

    # 兼容旧值 COMPOSE_FILE=docker-compose.prod.yml（文件已迁到 deploy/）。
    if composed.name == BASE_COMPOSE_FILE and composed.parent == deploy_path:
        preferred = deploy_path / "deploy" / composed.name
        if preferred.is_file():
            composed = preferred.resolve()
    if not composed.is_file():
        legacy = deploy_path / "deploy" / composed.name
        if legacy.is_file():
            composed = legacy.resolve()
    return composed


def _expected_containers(mode: str, profiles: Sequence[str]) -> Tuple[str, ...]:
    containers = list(_APP_CONTAINERS)
    for profile in profiles:
        containers.extend(_PROFILE_CONTAINERS.get(profile, ()))
    if mode == "docker":
        containers.append("forklift-pg-prod")
    return tuple(dict.fromkeys(containers))


def database_topology_for_mode(
    mode: str,
    compose_files: Tuple[Path, ...],
    profiles: Sequence[str],
) -> DatabaseAdapter:
    if mode == "docker":
        return DockerDatabaseAdapter(
            compose_files=compose_files,
            expected_containers=_expected_containers(mode, profiles),
        )
    if mode == "external":
        return ExternalDatabaseAdapter(
            compose_files=compose_files,
            expected_containers=_expected_containers(mode, profiles),
        )
    raise ValueError("DB_MODE 必须为 docker 或 external")


def resolve_topology(
    *,
    config: EffectiveConfig,
    environment: Mapping[str, str],
    deploy_path: Path,
    profiles: Sequence[str] = (),
    compose_file: Optional[Path] = None,
) -> DeploymentTopology:
    """从 EffectiveConfig 与环境输入解析当前部署拓扑。"""
    deploy_path = Path(deploy_path).resolve()
    resolved = config.resolve(environment=dict(environment), overrides={})
    deploy_environment = (environment.get("DEPLOY_ENVIRONMENT") or "production").strip()
    if deploy_environment not in ("testing", "production"):
        raise ValueError("DEPLOY_ENVIRONMENT 必须为 testing 或 production")
    raw_mode = (environment.get("DB_MODE") or "").strip()
    mode = raw_mode or ("docker" if deploy_environment == "testing"
                        else (resolved.get("DB_MODE") or "docker").strip())
    if mode not in ("docker", "external"):
        raise ValueError("DB_MODE 必须为 docker 或 external")

    postgres_image = environment.get("POSTGRES_IMAGE") or ""
    if not postgres_image:
        postgres_image = ("postgres:19.0-trixie" if deploy_environment == "testing"
                          else resolved.get("POSTGRES_IMAGE") or "postgres:18-alpine")
    pg_volume = environment.get("PG_VOLUME") or ""
    if not pg_volume:
        pg_volume = ("pgdata-testing-19"
                     if deploy_environment == "testing" and postgres_image == "postgres:19.0-trixie"
                     else resolved.get("PG_VOLUME") or "pgdata-prod")

    if compose_file is not None:
        base = Path(compose_file)
        base = (base if base.is_absolute() else deploy_path / base).resolve()
    else:
        base = _base_compose_file(resolved, deploy_path)

    compose_files = (base,)
    if mode == "external":
        overlay = base.with_name(EXTERNAL_COMPOSE_FILE)
        if overlay != base:
            compose_files += (overlay,)

    if mode == "external" and "retained-postgres" in profiles:
        raise ValueError("外部数据库模式不能启用保留的 PostgreSQL 容器")

    return DeploymentTopology(
        schema_version=SCHEMA_VERSION,
        release_id=resolve_release_id(resolved),
        database=database_topology_for_mode(mode, compose_files, tuple(profiles)),
        postgres_image=postgres_image,
        pg_volume=pg_volume,
    )


class TopologySnapshotError(ValueError):
    """拓扑快照缺失、损坏或与当前发布不一致。"""


def snapshot_path_for(deploy_path: Path) -> Path:
    return Path(deploy_path) / SNAPSHOT_RELATIVE_PATH


def current_release_id_path_for(deploy_path: Path) -> Path:
    return Path(deploy_path) / CURRENT_RELEASE_ID_RELATIVE_PATH


def _fsync_directory(path: Path) -> None:
    try:
        fd = os.open(path, os.O_RDONLY)
    except OSError:
        return
    try:
        os.fsync(fd)
    except OSError:
        pass
    finally:
        os.close(fd)


def _atomic_write_text(path: Path, text: str) -> None:
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    fd, temporary = tempfile.mkstemp(
        prefix=f".{path.name}.",
        suffix=".tmp",
        dir=str(path.parent),
    )
    try:
        with os.fdopen(fd, "w", encoding="utf-8", newline="\n") as stream:
            stream.write(text)
            stream.flush()
            os.fsync(stream.fileno())
        os.chmod(temporary, 0o600)
        os.replace(temporary, path)
        _fsync_directory(path.parent)
    except Exception:
        try:
            os.unlink(temporary)
        except FileNotFoundError:
            pass
        raise


def write_topology_snapshot(path: Path, topology: DeploymentTopology) -> None:
    """先原子发布快照，再原子发布当前发布标记；两步间失败必然校验失败。"""
    payload = topology.to_dict()
    _validate_snapshot(payload)
    path = Path(path)
    serialized = json.dumps(payload, ensure_ascii=False, indent=2, sort_keys=True) + "\n"
    _atomic_write_text(path, serialized)
    _atomic_write_text(current_release_id_path_for(path.parent.parent), topology.release_id + "\n")


def _require_exact_keys(value: object, keys: set[str], field: str) -> Dict[str, object]:
    if not isinstance(value, dict):
        raise TopologySnapshotError(f"快照字段 {field} 必须是对象")
    if set(value) != keys:
        raise TopologySnapshotError(f"快照字段 {field} 结构不正确")
    return value


def _validate_snapshot(
    payload: object,
    expected_release_id: Optional[str] = None,
) -> Dict[str, object]:
    top_keys = {
        "schema_version",
        "release_id",
        "database_mode",
        "postgres_image",
        "pg_volume",
        "compose_files",
        "expected_containers",
        "migration",
        "backup",
        "watchdog",
    }
    data = _require_exact_keys(payload, top_keys, "root")

    schema_version = data["schema_version"]
    if isinstance(schema_version, bool) or schema_version != SCHEMA_VERSION:
        raise TopologySnapshotError(f"快照 schema_version 不受支持: {schema_version!r}")

    release_id = data["release_id"]
    if not isinstance(release_id, str) or not _SAFE_RELEASE_ID.fullmatch(release_id):
        raise TopologySnapshotError("快照缺少有效 release_id")
    if expected_release_id is not None and release_id != expected_release_id:
        raise TopologySnapshotError(
            f"快照发布标识不匹配: 期望 {expected_release_id!r}, 实际 {release_id!r}"
        )

    mode = data["database_mode"]
    if mode not in ("docker", "external"):
        raise TopologySnapshotError(f"快照 database_mode 不受支持: {mode!r}")

    for field in ("postgres_image", "pg_volume"):
        value = data[field]
        if not isinstance(value, str) or not value.strip():
            raise TopologySnapshotError(f"快照缺少有效 {field}")

    compose_files = data["compose_files"]
    if (not isinstance(compose_files, list) or not compose_files
            or any(not isinstance(item, str) or not item for item in compose_files)):
        raise TopologySnapshotError("快照缺少 compose_files")
    compose_names = [Path(item).name for item in compose_files]
    if mode == "docker":
        if EXTERNAL_COMPOSE_FILE in compose_names:
            raise TopologySnapshotError("docker 快照不能包含 external compose overlay")
    elif len(compose_names) < 2 or compose_names[-1] != EXTERNAL_COMPOSE_FILE:
        raise TopologySnapshotError("external 快照缺少 external compose overlay")

    containers = data["expected_containers"]
    if (not isinstance(containers, list) or not containers
            or any(not isinstance(item, str) or not _CONTAINER_NAME.fullmatch(item)
                   for item in containers)):
        raise TopologySnapshotError("快照缺少有效 expected_containers")
    required_containers = {
        "forklift-backend-prod",
        "forklift-frontend-prod",
        "forklift-open-school-prod",
    }
    if not required_containers.issubset(containers):
        raise TopologySnapshotError("快照缺少必需的应用容器结论")
    if mode == "docker" and "forklift-pg-prod" not in containers:
        raise TopologySnapshotError("docker 快照缺少 PostgreSQL 容器结论")
    if mode == "external" and "forklift-pg-prod" in containers:
        raise TopologySnapshotError("external 快照不能包含 PostgreSQL 容器结论")

    migration = _require_exact_keys(data["migration"], {"entrypoint", "command"}, "migration")
    command = migration["command"]
    if (migration["entrypoint"] not in ("docker_run", "compose_run")
            or command != ["/app/bin/migrate", "up"]):
        raise TopologySnapshotError("快照缺少有效 migration 结论")
    if mode == "docker" and migration["entrypoint"] != "docker_run":
        raise TopologySnapshotError("docker 快照迁移入口不一致")
    if mode == "external" and migration["entrypoint"] != "compose_run":
        raise TopologySnapshotError("external 快照迁移入口不一致")

    backup = _require_exact_keys(data["backup"], {"strategy"}, "backup")
    watchdog = _require_exact_keys(data["watchdog"], {"strategy"}, "watchdog")
    if mode == "docker":
        if backup["strategy"] != "docker_exec" or watchdog["strategy"] != "docker_postgres":
            raise TopologySnapshotError("docker 快照备份/巡检策略不一致")
    elif backup["strategy"] != "external_pg_dump" or watchdog["strategy"] != "external_postgres_tls":
        raise TopologySnapshotError("external 快照备份/巡检策略不一致")

    return data


def read_topology_snapshot(
    path: Path,
    expected_release_id: Optional[str] = None,
) -> Dict[str, object]:
    """读取并校验快照与当前发布标记；任何缺失/损坏/不匹配均 fail closed。"""
    path = Path(path)
    marker_path = current_release_id_path_for(path.parent.parent)
    try:
        raw_marker = marker_path.read_text(encoding="utf-8")
    except (OSError, UnicodeError) as exc:
        raise TopologySnapshotError(f"无法读取当前发布标记: {marker_path}: {exc}") from exc
    marker_release_id = raw_marker.strip()
    if not _SAFE_RELEASE_ID.fullmatch(marker_release_id):
        raise TopologySnapshotError("当前发布标记损坏或缺少有效 release_id")
    if expected_release_id is not None and marker_release_id != expected_release_id:
        raise TopologySnapshotError(
            f"当前发布标记不匹配: 期望 {expected_release_id!r}, 实际 {marker_release_id!r}"
        )
    try:
        raw = path.read_text(encoding="utf-8")
    except (OSError, UnicodeError) as exc:
        raise TopologySnapshotError(f"无法读取拓扑快照: {path}: {exc}") from exc
    try:
        payload = json.loads(raw)
    except json.JSONDecodeError as exc:
        raise TopologySnapshotError(f"拓扑快照损坏: {path}: {exc}") from exc
    return _validate_snapshot(payload, expected_release_id=marker_release_id)
