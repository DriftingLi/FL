"""ReleaseTransaction module：部署事务的 plan → execute → rollback → verify。

从 deploy.py Deployer 抽出的部署生命周期 module。核心变更：
- plan 构造时生成 immutable 计划（全部服务含 libreoffice，覆盖完整生命周期）
- 每个服务条目记录 previous/target 双状态
- execute 按计划推进，rollback 基于同一计划反转
- 可变状态（applied / backup_error）归入 Transaction 内部
- Environment interface 注入 Docker/IO 操作，测试用 fake

兼容 ADR-0088 决策 3/5/6（显式变更计划、迁移先于重启、回滚只动变化服务）。
"""
from __future__ import annotations

import json
from dataclasses import dataclass, replace
from pathlib import Path
from typing import Callable, Dict, List, Mapping, Optional, Protocol, Sequence, Tuple

from topology import (
    _atomic_write_text,
    _validate_snapshot,
    read_topology_snapshot,
    snapshot_path_for,
)


# ---------------------------------------------------------------- 类型定义

@dataclass(frozen=True)
class ServiceChange:
    """单个服务的变更条目：previous → target 双状态。"""
    service: str
    previous_image: str   # "unknown" 表示未运行
    target_image: str
    action: str           # skip | start | create | recreate
    reason: str

    @property
    def changes_container(self) -> bool:
        return self.action in ("start", "create", "recreate")


@dataclass(frozen=True)
class ReleasePlan:
    """不可变的部署变更计划，覆盖完整生命周期。"""
    services: Tuple[ServiceChange, ...]
    migration_needed: bool = False
    release_id: str = ""
    previous_release_id: Optional[str] = None
    previous_topology: Optional[Dict[str, object]] = None
    target_topology: Optional[Dict[str, object]] = None
    migration: Optional[Dict[str, object]] = None
    persisted_applied_services: Tuple[str, ...] = ()

    @property
    def applied_services(self) -> Tuple[ServiceChange, ...]:
        """action != skip 的服务（需要实际变更的）。"""
        return tuple(s for s in self.services if s.changes_container)


class Environment(Protocol):
    """ReleaseTransaction 依赖的环境操作接口（Docker 查询 / 迁移 / 健康检查）。

    实现方负责实际的 Docker / DB / HTTP 操作；测试注入 fake 记录调用序。
    """

    def get_running_image(self, service: str) -> str:
        """返回服务当前运行镜像坐标；未运行返回 "unknown"。"""
        ...

    def recreate_service(self, service: str, force: bool = False) -> None:
        """重建指定服务容器。force=True 时带 --force-recreate（回滚必用）。"""
        ...

    def stop_service(self, service: str) -> None:
        """停止部署前不存在的服务，恢复到 previous_image=unknown。"""
        ...

    def set_service_image(self, service: str, image: str) -> None:
        """设置服务的目标镜像坐标（用于回滚时切回 previous_image）。"""
        ...

    def verify_running_image(self, service: str, expected: str) -> bool:
        """验证服务运行镜像与期望一致（回滚自证）。"""
        ...

    def preflight_migration_check(self, image: str) -> bool:
        """迁移预检：镜像的迁移版本是否兼容当前数据库。"""
        ...

    def fix_dirty_state(self) -> None:
        """清理 dirty 迁移状态。"""
        ...

    def run_migration(self) -> None:
        """执行数据库迁移。"""
        ...

    def health_check(self) -> None:
        """健康检查（全部服务）。"""
        ...


# ---------------------------------------------------------------- plan 构造

def build_release_plan(
    env: Environment,
    desired_images: Dict[str, str],
    decisions: Optional[List] = None,
    migration_needed: bool = False,
) -> ReleasePlan:
    """构造不可变的部署变更计划。

    Args:
        env: 环境接口（查询当前运行镜像）
        desired_images: 期望的镜像坐标 {service: image_ref}
        decisions: 可选的 decide_service 结果列表（为 None 时全部按 recreate 处理）
        migration_needed: 是否需要数据库迁移
    """
    # 构建 action 映射：如果给了 decisions 就用，否则全部 recreate
    action_map: Dict[str, Tuple[str, str]] = {}
    if decisions:
        for d in decisions:
            action_map[d.service] = (d.action, d.reason)

    services: List[ServiceChange] = []
    for service, target_image in desired_images.items():
        previous_image = env.get_running_image(service)
        if service in action_map:
            action, reason = action_map[service]
        else:
            # 无显式 decision 时：有变更就 recreate，无变更 skip
            if previous_image == target_image:
                action, reason = "skip", "无变化"
            else:
                action = "recreate" if previous_image != "unknown" else "create"
                reason = f"镜像变更: {previous_image} → {target_image}"

        services.append(ServiceChange(
            service=service,
            previous_image=previous_image,
            target_image=target_image,
            action=action,
            reason=reason,
        ))

    return ReleasePlan(
        services=tuple(services),
        migration_needed=migration_needed,
    )


# ---------------------------------------------------------------- Transaction

class ReleaseTransaction:
    """部署事务：拥有 plan → execute → rollback → verify 完整生命周期。

    可变状态（applied 列表、backup_error）归入内部，Deployer 不再是状态容器。
    """

    def __init__(
        self,
        plan: ReleasePlan,
        env: Environment,
        on_applied: Optional[Callable[[Sequence[str]], None]] = None,
    ):
        self._plan = plan
        self._env = env
        self._applied: List[str] = []
        self._backup_error: Optional[str] = None
        self._on_applied = on_applied

    @classmethod
    def restore(cls, plan: ReleasePlan, env: Environment) -> "ReleaseTransaction":
        """从版本化计划恢复 transaction；已应用服务以持久化进度为准。"""
        txn = cls(plan, env)
        txn._applied.extend(plan.persisted_applied_services)
        if not plan.release_id:
            txn._applied.extend(change.service for change in plan.applied_services)
        return txn

    @property
    def plan(self) -> ReleasePlan:
        return self._plan

    @property
    def applied_services(self) -> List[str]:
        return list(self._applied)

    @property
    def backup_error(self) -> Optional[str]:
        return self._backup_error

    @backup_error.setter
    def backup_error(self, value: Optional[str]) -> None:
        self._backup_error = value

    def execute(self) -> None:
        """按计划推进部署：清 dirty → 迁移（如需）→ 服务变更 → 健康检查。"""
        # dirty 状态与「是否有待跑迁移」是两件事：即使版本已最新也要清 dirty。
        self._env.fix_dirty_state()
        # 迁移先于服务重启（ADR-0088 决策 5）
        if self._plan.migration_needed:
            self._env.run_migration()

        # 服务变更
        for change in self._plan.services:
            if change.changes_container:
                # 先记尝试意图再动容器：进程在 recreate 中途退出时，手工回滚仍不会漏掉它。
                self._applied.append(change.service)
                if self._on_applied is not None:
                    self._on_applied(self.applied_services)
                self._env.recreate_service(change.service)

        # 健康检查
        self._env.health_check()

    def rollback(self) -> None:
        """反转已应用服务；部分失败继续，最终统一 verify previous 状态。"""
        if not self._applied:
            return

        changes = [c for c in reversed(self._plan.services) if c.service in self._applied]
        failures: List[str] = []
        backend = next((c for c in changes if c.service == "backend"), None)
        backend_preflight_failed = False
        if backend is not None and backend.previous_image != "unknown":
            try:
                compatible = self._env.preflight_migration_check(backend.previous_image)
            except Exception as exc:  # noqa: BLE001 — 预检异常也汇入总体失败
                compatible = False
                failures.append(f"backend migration preflight: {exc}")
            if not compatible:
                backend_preflight_failed = True
                failures.append(
                    f"拒绝回滚后端到 {backend.previous_image}（迁移版本落后数据库，会崩溃循环）"
                )

        for change in changes:
            if change.service == "backend" and backend_preflight_failed:
                continue
            try:
                if change.previous_image == "unknown":
                    self._env.stop_service(change.service)
                else:
                    self._env.set_service_image(change.service, change.previous_image)
                    self._env.recreate_service(change.service, force=True)
            except Exception as exc:  # noqa: BLE001 — 汇总服务失败，不中断其余回滚
                failures.append(f"{change.service}: {exc}")

        try:
            self._env.health_check()
        except Exception as exc:  # noqa: BLE001
            failures.append(f"health_check: {exc}")

        for change in changes:
            expected = change.previous_image
            try:
                if not self._env.verify_running_image(change.service, expected):
                    failures.append(f"{change.service}: 期望 {expected}，运行镜像不匹配")
            except Exception as exc:  # noqa: BLE001
                failures.append(f"{change.service}: verify: {exc}")

        if failures:
            raise RuntimeError("回滚未完成：" + "；".join(failures))

    def verify(self) -> None:
        """校验所有 apply 过的服务运行镜像 = target_image；不一致时 raise。"""
        for change in self._plan.services:
            if change.service in self._applied:
                if not self._env.verify_running_image(change.service, change.target_image):
                    raise RuntimeError(
                        f"服务 {change.service} 验证失败："
                        f"期望 {change.target_image}，实际镜像不匹配",
                    )


# ---------------------------------------------------------------- 版本化发布计划

SCHEMA_VERSION = 1
RELEASE_PLAN_RELATIVE_PATH = Path("backups") / "release-plan.json"


class ReleasePlanError(ValueError):
    """发布计划缺失、损坏、缺字段或版本不兼容。"""


def release_plan_path_for(deploy_path: Path) -> Path:
    return Path(deploy_path) / RELEASE_PLAN_RELATIVE_PATH


def _safe_image(value: object, *, allow_unknown: bool = False) -> bool:
    if not isinstance(value, str) or not value or any(c.isspace() for c in value):
        return False
    return (allow_unknown and value == "unknown") or (value != "unknown" and ":" in value.rsplit("/", 1)[-1])


def _image_tag(image: str) -> str:
    if not _safe_image(image) or "@" in image:
        raise ReleasePlanError(f"无法从历史镜像可靠推导 release_id: {image!r}")
    return image.rsplit(":", 1)[1]


def _validate_plan(plan: ReleasePlan) -> None:
    if not isinstance(plan.release_id, str) or not plan.release_id or any(c.isspace() for c in plan.release_id):
        raise ReleasePlanError("发布计划缺少有效 release_id")
    if plan.previous_release_id is not None and not isinstance(plan.previous_release_id, str):
        raise ReleasePlanError("发布计划 previous_release_id 无效")
    if not plan.services:
        raise ReleasePlanError("发布计划缺少 services")
    names = [change.service for change in plan.services]
    if any(not isinstance(name, str) or not name for name in names):
        raise ReleasePlanError("发布计划包含无效服务名")
    if len(names) != len(set(names)):
        raise ReleasePlanError("发布计划包含重复服务")
    for change in plan.services:
        if not _safe_image(change.target_image):
            raise ReleasePlanError(f"服务 {change.service!r} 缺少有效 target_image")
        if not _safe_image(change.previous_image, allow_unknown=True):
            raise ReleasePlanError(f"服务 {change.service!r} 缺少有效 previous_image")
        if change.action not in ("skip", "start", "create", "recreate") or not isinstance(change.reason, str):
            raise ReleasePlanError(f"服务 {change.service!r} 的 action/reason 无效")
    known_previous = [c for c in plan.services if c.previous_image != "unknown"]
    if known_previous and not plan.previous_release_id:
        raise ReleasePlanError("存在历史镜像但缺少 previous_release_id")
    if not known_previous and plan.previous_release_id is not None:
        raise ReleasePlanError("没有历史镜像但声明了 previous_release_id，拒绝伪造上一版本")
    if (plan.previous_release_id is None) != (plan.previous_topology is None):
        raise ReleasePlanError("previous_release_id 与 previous_topology 必须同时为空或同时存在")
    if plan.previous_release_id == plan.release_id:
        raise ReleasePlanError("previous_release_id 不能等于 release_id")
    target_topology = plan.target_topology
    if target_topology is None:
        raise ReleasePlanError("发布计划缺少 target_topology")
    try:
        _validate_snapshot(target_topology)
    except ValueError as exc:
        raise ReleasePlanError(f"target_topology 无效: {exc}") from exc
    if target_topology["release_id"] != plan.release_id:
        raise ReleasePlanError("target_topology.release_id 与计划不一致")
    if plan.previous_release_id is not None:
        try:
            _validate_snapshot(plan.previous_topology)
        except ValueError as exc:
            raise ReleasePlanError(f"previous_topology 无效: {exc}") from exc
        if plan.previous_topology["release_id"] != plan.previous_release_id:
            raise ReleasePlanError("previous_topology.release_id 与计划不一致")
    migration = plan.migration
    if (not isinstance(migration, dict)
            or set(migration) != {"needed", "preflight_verified", "previous_backend_image", "target_backend_image"}
            or not isinstance(migration["needed"], bool)
            or not isinstance(migration["preflight_verified"], bool)
            or migration["preflight_verified"] is not True):
        raise ReleasePlanError("发布计划缺少已验证的 migration 边界")
    if migration["needed"] is not plan.migration_needed:
        raise ReleasePlanError("migration 边界与 migration_needed 不一致")
    for key in ("previous_backend_image", "target_backend_image"):
        if not _safe_image(migration[key], allow_unknown=True):
            raise ReleasePlanError(f"migration.{key} 无效")
    applied = set(plan.persisted_applied_services)
    if not applied.issubset(names):
        raise ReleasePlanError("持久化 applied_services 包含未知服务")


def save_release_plan(path: Path, plan: ReleasePlan) -> None:
    _validate_plan(plan)
    payload = {
        "schema_version": SCHEMA_VERSION,
        "release_id": plan.release_id,
        "previous_release_id": plan.previous_release_id,
        "previous_topology": plan.previous_topology,
        "target_topology": plan.target_topology,
        "migration": plan.migration,
        "applied_services": list(plan.persisted_applied_services),
        "services": [
            {
                "service": c.service,
                "previous_image": c.previous_image,
                "target_image": c.target_image,
                "action": c.action,
                "reason": c.reason,
            }
            for c in plan.services
        ],
    }
    try:
        _atomic_write_text(path, json.dumps(payload, ensure_ascii=False, indent=2, sort_keys=True) + "\n")
    except OSError as exc:
        raise ReleasePlanError(f"无法持久化发布计划: {path}: {exc}") from exc


def load_release_plan(path: Path) -> ReleasePlan:
    try:
        payload = json.loads(Path(path).read_text(encoding="utf-8"))
    except (OSError, UnicodeError, json.JSONDecodeError) as exc:
        raise ReleasePlanError(f"无法读取发布计划: {path}: {exc}") from exc
    keys = {
        "schema_version", "release_id", "previous_release_id", "previous_topology",
        "target_topology", "migration", "applied_services", "services",
    }
    if not isinstance(payload, dict) or set(payload) != keys:
        raise ReleasePlanError("发布计划根字段缺失或结构不正确")
    if payload["schema_version"] != SCHEMA_VERSION or isinstance(payload["schema_version"], bool):
        raise ReleasePlanError(f"发布计划 schema_version 不受支持: {payload['schema_version']!r}")
    raw_services = payload["services"]
    if not isinstance(raw_services, list):
        raise ReleasePlanError("发布计划 services 必须是数组")
    services = []
    for raw in raw_services:
        if not isinstance(raw, dict) or set(raw) != {"service", "previous_image", "target_image", "action", "reason"}:
            raise ReleasePlanError("发布计划服务条目缺少字段")
        services.append(ServiceChange(**raw))
    applied = payload["applied_services"]
    if not isinstance(applied, list) or any(not isinstance(item, str) for item in applied):
        raise ReleasePlanError("发布计划 applied_services 无效")
    plan = ReleasePlan(
        services=tuple(services),
        migration_needed=payload["migration"].get("needed", False) if isinstance(payload["migration"], dict) else False,
        release_id=payload["release_id"],
        previous_release_id=payload["previous_release_id"],
        previous_topology=payload["previous_topology"],
        target_topology=payload["target_topology"],
        migration=payload["migration"],
        persisted_applied_services=tuple(applied),
    )
    _validate_plan(plan)
    return plan


def create_release_plan(
    *,
    env,
    desired_images: Mapping[str, str],
    decisions: Optional[Sequence],
    migration_needed: bool,
    migration_preflight_verified: bool,
    release_id: str,
    topology,
    path: Path,
    last_version_path: Path,
    previous_images: Optional[Mapping[str, str]] = None,
    fallback_previous_topology: Optional[Mapping[str, object]] = None,
) -> ReleasePlan:
    """在变更前构造并原子落盘；上一份计划是 previous 镜像与拓扑的事实源。"""
    path = Path(path)
    previous_plan = load_release_plan(path) if path.exists() else None
    if previous_plan is not None:
        check_persisted_topology(path.parent.parent, previous_plan)
    plan = build_release_plan(env, dict(desired_images), decisions, migration_needed)
    if previous_images is not None:
        plan = replace(plan, services=tuple(
            replace(c, previous_image=previous_images[c.service]) for c in plan.services
        ))
    known_previous = [c for c in plan.services if c.previous_image != "unknown"]
    if previous_plan is not None and known_previous:
        previous_id = previous_plan.release_id
        previous_topology = previous_plan.target_topology
    elif not known_previous:
        previous_id = None
        previous_topology = None
    else:
        backend = next((c for c in plan.services if c.service == "backend"), None)
        if backend is None or backend.previous_image == "unknown":
            raise ReleasePlanError("backend previous_image 未知，拒绝猜测 previous_release_id")
        previous_id = _image_tag(backend.previous_image)
        if fallback_previous_topology is None:
            raise ReleasePlanError(
                "没有上一份发布计划且缺少可用保存环境，拒绝从历史镜像猜测 previous_topology"
            )
        previous_topology = {**fallback_previous_topology, "release_id": previous_id}
    target_topology = topology.to_dict()
    backend = next((c for c in plan.services if c.service == "backend"), None)
    migration = {
        "needed": migration_needed,
        "preflight_verified": migration_preflight_verified,
        "previous_backend_image": backend.previous_image if backend else "unknown",
        "target_backend_image": backend.target_image if backend else "unknown",
    }
    plan = replace(
        plan,
        release_id=release_id,
        previous_release_id=previous_id,
        previous_topology=previous_topology,
        target_topology=target_topology,
        migration=migration,
    )
    save_release_plan(path, plan)
    _write_last_version(last_version_path, plan.services)
    return plan


def persist_applied_services(path: Path, plan: ReleasePlan, applied_services: Sequence[str]) -> ReleasePlan:
    updated = replace(plan, persisted_applied_services=tuple(applied_services))
    save_release_plan(path, updated)
    return updated


def rollback_plan(plan: ReleasePlan, path: Path, last_version_path: Path) -> ReleasePlan:
    """成功回滚后把 previous 发布立为当前计划，保留反向恢复事实。"""
    if not plan.previous_release_id:
        raise ReleasePlanError("没有 previous_release_id，无法生成回滚后的发布计划")
    backend = next((c for c in plan.services if c.service == "backend"), None)
    migration = {
        "needed": False,
        "preflight_verified": True,
        "previous_backend_image": backend.target_image if backend else "unknown",
        "target_backend_image": backend.previous_image if backend else "unknown",
    }
    restored = replace(
        plan,
        release_id=plan.previous_release_id,
        previous_release_id=plan.release_id,
        previous_topology=plan.target_topology,
        target_topology=plan.previous_topology,
        migration=migration,
        migration_needed=False,
        services=tuple(replace(c, previous_image=c.target_image, target_image=c.previous_image,
                               action="recreate", reason="application rollback") for c in plan.services),
    )
    save_release_plan(Path(path), restored)
    _write_last_version(Path(last_version_path), restored.services)
    return restored


def _write_last_version(path: Path, services: Sequence[ServiceChange]) -> None:
    text = "".join(f"PREVIOUS_{c.service.upper()}_IMAGE={c.previous_image}\n" for c in services)
    try:
        _atomic_write_text(path, text)
    except OSError as exc:
        raise ReleasePlanError(f"无法更新兼容回滚指针: {path}: {exc}") from exc


def check_persisted_topology(deploy_path: Path, plan: ReleasePlan) -> None:
    """已有快照时必须与上一份计划一致，防止拿过期计划继续发布。"""
    snapshot = snapshot_path_for(deploy_path)
    if not snapshot.exists():
        raise ReleasePlanError("已有发布计划但缺少当前拓扑快照，拒绝继续发布")
    try:
        current = read_topology_snapshot(snapshot)
    except ValueError as exc:
        raise ReleasePlanError(f"当前拓扑快照无效: {exc}") from exc
    if current["release_id"] != plan.release_id:
        raise ReleasePlanError("当前拓扑快照与发布计划不一致；如有未完成发布，请先显式回滚")
    if plan.target_topology != current:
        raise ReleasePlanError("发布计划的目标拓扑引用与当前快照不一致")
