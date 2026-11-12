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

from dataclasses import dataclass, field
from typing import Dict, List, Optional, Protocol, Tuple


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

    def __init__(self, plan: ReleasePlan, env: Environment):
        self._plan = plan
        self._env = env
        self._applied: List[str] = []
        self._backup_error: Optional[str] = None

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
                self._env.recreate_service(change.service)
                self._applied.append(change.service)

        # 健康检查
        self._env.health_check()

    def rollback(self) -> None:
        """基于同一 plan 反转：只回滚实际 apply 过的服务，恢复到 previous_image。

        回滚只动变化服务（ADR-0088 决策 6）。
        """
        if not self._applied:
            return

        # 反转序：后部署的先回滚；先设镜像再重建（否则 recreate 会拉当前镜像）
        for change in reversed(self._plan.services):
            if change.service in self._applied and change.previous_image != "unknown":
                # 后端迁移预检：拒绝回滚到迁移落后于数据库的镜像（防崩溃循环）
                if change.service == "backend":
                    if not self._env.preflight_migration_check(change.previous_image):
                        raise RuntimeError(
                            f"拒绝回滚后端到 {change.previous_image}"
                            f"（迁移版本落后数据库，会崩溃循环）"
                        )
                self._env.set_service_image(change.service, change.previous_image)
                self._env.recreate_service(change.service, force=True)

        self._env.health_check()

    def verify(self) -> None:
        """校验所有 apply 过的服务运行镜像 = target_image；不一致时 raise。"""
        for change in self._plan.services:
            if change.service in self._applied:
                if not self._env.verify_running_image(change.service, change.target_image):
                    raise RuntimeError(
                        f"服务 {change.service} 验证失败："
                        f"期望 {change.target_image}，实际镜像不匹配",
                    )
