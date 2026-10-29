#!/usr/bin/env python3
"""deploy/deploy.py — 叉车维修培训系统部署 CLI（远程 CD / 本地手动共用）。

替代旧的 scripts/deploy-remote.sh 与根 deploy.sh。设计口径见
docs/adr/ADR-0088-部署脚本收敛与按变更重启.md：

* **按变更重启**：先算计划（容器状态 + 镜像内容 + compose config-hash），
  只对真正变化的服务执行 up；postgres 是数据服务，默认永不自动重建。
* **迁移先于应用重启**：新后端镜像先跑 migrate，成功后才重建 backend / frontend。
* **回滚只动回滚涉及的服务**，并做运行镜像自证。
* 纯标准库、Python 3.9+；目标服务器（Debian / PVE）自带 python3，不依赖 Node。

用法（服务器上，CD 就是这么调用的）：
    python3 deploy/deploy.py                 # 部署（默认模式）
    python3 deploy/deploy.py --plan          # 只打印变更计划，不重建运行中的容器
    python3 deploy/deploy.py --plan --json   # 机器可读计划
    python3 deploy/deploy.py rollback        # 回滚到 backups/last-version.txt
    python3 deploy/deploy.py down            # 停止本 compose 项目（testing 冒烟收尾）
    python3 deploy/deploy.py local [--skip-build] [--skip-migrate]
                                             # 本地 / 手动一键部署

只做判定的 dry-run 入口（不碰 docker、不写文件，常驻判据物）：
    python3 deploy/deploy.py --migration-gate
    python3 deploy/deploy.py --rollback-plan
    python3 deploy/deploy.py --image-match <actual> <expected>
"""

from __future__ import annotations

import argparse
import gzip
import json
import os
import re
import shlex
import shutil
import subprocess
import sys
import tempfile
import threading
import time
from dataclasses import dataclass
from pathlib import Path
from typing import Dict, List, Optional
from urllib.parse import quote

RED = "\033[0;31m"
GREEN = "\033[0;32m"
YELLOW = "\033[1;33m"
BLUE = "\033[0;34m"
NC = "\033[0m"

REPO_ROOT = Path(__file__).resolve().parent.parent
# 日志流：--json 时改走 stderr，保证 stdout 只有 JSON。
LOG_STREAM = sys.stdout

# 数据服务：只确保在跑，绝不因为应用镜像变更而重建（#2109 起 Redis 已退役，只剩 postgres）。
DATA_SERVICES = ("postgres",)
APP_SERVICES = ("libreoffice", "backend", "frontend")
# compose 中显式指定的容器名：用于清理同名但非本项目管理的残留容器（沿用旧脚本行为）。
KNOWN_CONTAINERS = (
    "forklift-backend-prod",
    "forklift-frontend-prod",
    "forklift-pg-prod",
    "forklift-libreoffice-prod",
)


# ======================================================================
# 日志
# ======================================================================
def _configure_stdio() -> None:
    """统一 stdout/stderr 为 UTF-8（Windows GBK 控制台遇到 ✅ / 中文会 UnicodeEncodeError）。"""
    for stream in (sys.stdout, sys.stderr):
        try:
            stream.reconfigure(encoding="utf-8", errors="replace", newline="\n")  # type: ignore[attr-defined]
        except (AttributeError, ValueError):
            pass


def _log(level: str, color: str, msg: str) -> None:
    prefix = f"{color}[DEPLOY]{NC}" if sys.stdout.isatty() else "[DEPLOY]"
    print(f"{prefix} {time.strftime('%H:%M:%S')} {msg}", file=LOG_STREAM, flush=True)


def log_info(msg: str) -> None:
    _log("INFO", BLUE, msg)


def log_ok(msg: str) -> None:
    _log("OK", GREEN, f"✅ {msg}")


def log_warn(msg: str) -> None:
    _log("WARN", YELLOW, f"⚠️  {msg}")


def log_error(msg: str) -> None:
    _log("ERROR", RED, f"❌ {msg}")


class DeployError(RuntimeError):
    """部署流程的可预期失败：main 打印后返回 1。"""


# ======================================================================
# 纯判定（可被单元测试直接调用，不碰 docker / 文件系统）
# ======================================================================
def migration_failure_action(value: Optional[str]) -> str:
    """迁移失败后的去向（ADR-0056 §6 / #1099 的唯一判据点）。

    "1" = 带已知风险继续；其余（含未声明/空串）= 中止。
    """
    return "continue" if value == "1" else "abort"


def rollback_tag(image: Optional[str]) -> str:
    """从完整镜像坐标解析 tag；无 tag / unknown / 空 ⇒ 空串。"""
    image = (image or "").strip()
    if image in ("", "unknown"):
        return ""
    last = image.rsplit("/", 1)[-1]
    return last.rsplit(":", 1)[1] if ":" in last else ""


def images_equal(actual: Optional[str], expected: Optional[str]) -> bool:
    return (actual or "") == (expected or "")


@dataclass
class ServiceState:
    """docker inspect 里与部署判定相关的字段。"""

    running: bool
    image: str = ""
    image_id: str = ""
    config_hash: str = ""


@dataclass
class Decision:
    service: str
    action: str  # skip | start | create | recreate | blocked
    reason: str
    desired_image: str = ""

    @property
    def changes_container(self) -> bool:
        return self.action in ("start", "create", "recreate")


def decide_service(
    service: str,
    desired_image: str,
    state: Optional[ServiceState],
    expected_config_hash: str = "",
    desired_image_id: str = "",
    force: bool = False,
    data: bool = False,
    force_data: bool = False,
) -> Decision:
    """单个服务的动作判定（纯函数）。

    判定顺序有意如此：数据服务先看 config-hash —— 配置漂移（例如 PG_VOLUME 被改了）
    必须显式放行，不能因为在跑就静默忽略，也不能因为停了就静默换卷启动。
    """
    if state is not None and data and expected_config_hash and state.config_hash:
        if state.config_hash != expected_config_hash:
            if force_data:
                return Decision(service, "recreate", "显式 --force-data-services", desired_image)
            return Decision(service, "blocked", "数据服务配置变更（需 --force-data-services）", desired_image)

    if state is None:
        return Decision(service, "create", "容器不存在", desired_image)
    if not state.running:
        return Decision(service, "start", "容器未运行", desired_image)
    if force and not data:
        return Decision(service, "recreate", "显式 --force", desired_image)
    if force and data and force_data:
        return Decision(service, "recreate", "显式 --force-data-services", desired_image)

    if not data and desired_image:
        if state.image == desired_image:
            # 同一 tag 的镜像内容变了（latest 被覆盖 / 重新构建推送）：也要重建。
            image_changed = bool(desired_image_id and state.image_id and desired_image_id != state.image_id)
            reason = "镜像内容变更（同 tag 新镜像）"
        else:
            # tag 变了但镜像内容相同（镜像源 / 代理前缀不同）：不重建。
            image_changed = not (desired_image_id and state.image_id and desired_image_id == state.image_id)
            reason = f"镜像变更: {state.image or '?'} → {desired_image}"
        if image_changed:
            return Decision(service, "recreate", reason, desired_image)

    if expected_config_hash and state.config_hash and state.config_hash != expected_config_hash:
        return Decision(service, "recreate", "compose 配置变更", desired_image)

    return Decision(service, "skip", "无变化", desired_image)


def plan_decisions(
    desired_images: Dict[str, str],
    states: Dict[str, Optional[ServiceState]],
    config_hashes: Dict[str, str],
    desired_image_ids: Optional[Dict[str, str]] = None,
    force: bool = False,
    force_data: bool = False,
    services: Optional[List[str]] = None,
) -> List[Decision]:
    """把多个服务合成一个确定顺序的计划（纯函数，CI 判据物直接测它）。"""
    desired_image_ids = desired_image_ids or {}
    out: List[Decision] = []
    for service in services or list(states.keys()):
        out.append(
            decide_service(
                service,
                desired_images.get(service, ""),
                states.get(service),
                config_hashes.get(service, ""),
                desired_image_ids.get(service, ""),
                force=force,
                data=service in DATA_SERVICES,
                force_data=force_data,
            )
        )
    return out


# ======================================================================
# 环境变量：deploy/env.defaults 只作为「内存默认值」，绝不写进 os.environ
# ======================================================================
# 2026-09-13 生产事故的教训：部署链路不得用 source 无条件覆盖 CD 传入的值。
# 这里更进一步——默认值根本不进入进程环境，compose 子进程只能看到 CD 真正提供的变量，
# 镜像引用始终由 .env 单独控制（不会被 env.defaults 里的 BACKEND_IMAGE 盖掉）。
DEFAULTS: Dict[str, str] = {}

_ENV_LINE = re.compile(r"^export ([A-Za-z_][A-Za-z0-9_]*)=(.*)$")


def parse_env_defaults(text: str) -> Dict[str, str]:
    """解析 deploy/env.defaults 的文本（shell 可 source 格式），返回 {名字: 值}。"""
    out: Dict[str, str] = {}
    for line in text.splitlines():
        m = _ENV_LINE.match(line.strip())
        if not m:
            continue
        try:
            parts = shlex.split(m.group(2))
        except ValueError:
            continue
        out[m.group(1)] = parts[0] if parts else ""
    return out


def load_env_defaults(deploy_path: Path) -> Dict[str, str]:
    """读取 deploy/env.defaults；文件不存在时返回空表（手工部署仍可按 compose 默认值走）。"""
    path = Path(deploy_path) / "deploy" / "env.defaults"
    try:
        return parse_env_defaults(path.read_text(encoding="utf-8"))
    except OSError:
        return {}


def set_defaults(defaults: Dict[str, str]) -> None:
    global DEFAULTS
    DEFAULTS = dict(defaults)


def getenv(name: str, default: str = "") -> str:
    """取值优先级：真实环境（非空）→ env.defaults → 调用方默认值。"""
    value = os.environ.get(name, "")
    if value:
        return value
    value = DEFAULTS.get(name, "")
    return value if value else default


def env_int(name: str, default: int) -> int:
    try:
        return int(getenv(name, str(default)))
    except ValueError:
        return default


# ======================================================================
# 通用工具
# ======================================================================
def _strip_tag(image: str) -> str:
    last = image.rsplit("/", 1)[-1]
    return image.rsplit(":", 1)[0] if ":" in last else image


def resolve_image(primary_env: str, fallback_env: str, tag: str) -> str:
    """解析镜像引用：IMAGE_* 优先，缺省时回退到 env.defaults 的 *_IMAGE。"""
    repo = getenv(primary_env) or getenv(fallback_env)
    if not repo:
        return ""
    repo = _strip_tag(repo.lower())
    return f"{repo}:{tag}" if tag else repo


def env_val(value: str) -> str:
    """按 Docker Compose .env 语法转义（与旧脚本 env_val 同口径，额外转义反斜杠与引号）。"""
    if not value:
        return '""'
    if re.search(r"[\s\$+#{}()&|!<>'\";=\\]", value):
        escaped = value.replace("\\", "\\\\").replace('"', '\\"').replace("\n", "\\n")
        return f'"{escaped}"'
    return value


# ======================================================================
# 部署器
# ======================================================================
class Deployer:
    def __init__(self, mode: str, opts: argparse.Namespace):
        self.mode = mode
        self.opts = opts
        self.applied: List[str] = []
        self.previous_backend_image = "unknown"
        self.previous_frontend_image = "unknown"
        self.backup_error: Optional[str] = None

        # 部署根默认 = 本脚本所在仓库/部署目录的父级（远程布局里就是 $DEPLOY_PATH）。
        # CD 会显式导出 DEPLOY_PATH；手工执行时不再依赖写死的 /opt/forklift-training。
        self.deploy_path = (REPO_ROOT if mode == "local"
                            else Path(getenv("DEPLOY_PATH", str(REPO_ROOT))).resolve())
        # 必须在读其余配置前加载默认值；只进内存，不进 os.environ。
        set_defaults(load_env_defaults(self.deploy_path))

        raw_compose = os.environ.get("COMPOSE_FILE", "")
        if raw_compose:
            p = Path(raw_compose)
            composed = (p if p.is_absolute() else self.deploy_path / p).resolve()
        else:
            composed = (self.deploy_path / "deploy" / "docker-compose.prod.yml").resolve()
        # 兼容旧值 COMPOSE_FILE=docker-compose.prod.yml（文件已迁到 deploy/）：
        # 服务器上旧根文件可能还在，优先用 deploy/ 下的新文件，避免误用过期路径。
        if composed.name == "docker-compose.prod.yml" and composed.parent == self.deploy_path:
            preferred = self.deploy_path / "deploy" / composed.name
            if preferred.is_file():
                composed = preferred.resolve()
        if not composed.is_file():
            legacy = self.deploy_path / "deploy" / composed.name
            if legacy.is_file():
                composed = legacy.resolve()
        self.compose_file = composed

        raw_env = os.environ.get("ENV_FILE", "")
        if raw_env:
            p = Path(raw_env)
            self.env_file = (p if p.is_absolute() else self.deploy_path / p).resolve()
        elif mode == "local":
            candidate = self.deploy_path / "backend" / ".env"
            self.env_file = candidate if candidate.exists() else self.deploy_path / ".env"
        else:
            self.env_file = self.deploy_path / ".env"

        self.profiles = [p.strip() for p in getenv("COMPOSE_PROFILES").split(",") if p.strip()]
        self.libreoffice_enabled = "full" in self.profiles
        # 本地模式用仓库目录名当 compose 项目名（对齐旧 deploy.sh 的「按 cwd 命名」行为，
        # 也避免在开发机上和 forklift-training_* 生产卷撞名）。
        self.compose_project_args: List[str] = []
        if mode == "local":
            local_project = os.environ.get("COMPOSE_PROJECT_NAME") or REPO_ROOT.name.lower()
            local_project = re.sub(r"[^a-z0-9_-]", "", local_project.lower())
            if not local_project or not local_project[0].isalnum():
                local_project = "forklift-training"
            self.compose_project_args = ["--project-name", local_project]
        self.backup_dir = self.deploy_path / "backups"
        self.registry = getenv("REGISTRY", "ghcr.io")
        self.image_tag = getenv("IMAGE_TAG", "latest")
        self.tag_backend = getenv("IMAGE_TAG_BACKEND") or self.image_tag
        self.tag_frontend = getenv("IMAGE_TAG_FRONTEND") or self.image_tag
        self.tag_libreoffice = getenv("IMAGE_TAG_LIBREOFFICE") or self.image_tag
        self.registry_proxy = getenv("REGISTRY_PROXY").rstrip("/")
        self.registry_proxy_bind = getenv("REGISTRY_PROXY_BIND", "127.0.0.1")
        self.registry_mirror = getenv("REGISTRY_MIRROR", "ghcr.nju.edu.cn")
        self.github_token = getenv("GITHUB_TOKEN")
        self.backend_ref = resolve_image("IMAGE_BACKEND", "BACKEND_IMAGE", self.tag_backend)
        self.frontend_ref = resolve_image("IMAGE_FRONTEND", "FRONTEND_IMAGE", self.tag_frontend)
        self.libreoffice_ref = resolve_image("IMAGE_LIBREOFFICE", "LIBREOFFICE_IMAGE", self.tag_libreoffice)
        # 未改写前的引用：代理回退 / 镜像源回退 / 旧镜像清理都以它为基准。
        self.backend_ref_orig = self.backend_ref
        self.frontend_ref_orig = self.frontend_ref
        self.libreoffice_ref_orig = self.libreoffice_ref
        # 代理前缀改写（与旧脚本一致：.env 与镜像引用都用改写后的地址）。
        if self.registry_proxy:
            self.backend_ref = self._to_proxy(self.backend_ref)
            self.frontend_ref = self._to_proxy(self.frontend_ref)
            self.libreoffice_ref = self._to_proxy(self.libreoffice_ref)

        self.health_retries = env_int("HEALTH_CHECK_RETRIES", 20)
        self.health_interval = env_int("HEALTH_CHECK_INTERVAL", 6)
        self.skip_migration = getenv("SKIP_MIGRATION", "false").lower() == "true" or bool(opts.skip_migrate)
        self.skip_backup = getenv("SKIP_BACKUP", "false").lower() == "true"
        self.allow_migration_failure = getenv("ALLOW_MIGRATION_FAILURE")
        self.keep_images = env_int("KEEP_IMAGES", 3)
        self.cache_limit_mb = env_int("CACHE_LIMIT_MB", 15000)
        self.backup_wait_timeout = env_int("BACKUP_WAIT_TIMEOUT", 120)
        self.db_user = getenv("DB_USER", "forklift")
        self.db_password = getenv("DB_PASSWORD")
        self.db_name = "forklift_training"

    # ---------------------------------------------------------------- 基础
    def _to_proxy(self, image: str) -> str:
        if not image:
            return image
        if image.startswith("ghcr.io/") and self.registry_proxy:
            return f"{self.registry_proxy}/{image[len('ghcr.io/'):]}"
        return image

    def _rewrite_to_mirror(self) -> None:
        if not self.registry_mirror:
            return

        def rw(image: str) -> str:
            if image.startswith("ghcr.io/"):
                return f"{self.registry_mirror}/{image[len('ghcr.io/'):]}"
            return image

        self.backend_ref = rw(self.backend_ref_orig) or self.backend_ref
        self.frontend_ref = rw(self.frontend_ref_orig) or self.frontend_ref
        self.libreoffice_ref = rw(self.libreoffice_ref_orig) or self.libreoffice_ref

    def _compose(self) -> List[str]:
        cmd = ["docker", "compose"]
        if self.env_file.exists():
            cmd += ["--env-file", str(self.env_file)]
        cmd += ["-f", str(self.compose_file)]
        cmd += self.compose_project_args
        for profile in self.profiles:
            cmd += ["--profile", profile]
        return cmd

    def run(self, cmd: List[str], *, check: bool = True, capture: bool = False,
            timeout: Optional[int] = None, cwd: Optional[Path] = None) -> subprocess.CompletedProcess:
        try:
            cp = subprocess.run(cmd, text=True, capture_output=capture, timeout=timeout,
                                cwd=str(cwd) if cwd else None)
        except FileNotFoundError:
            raise DeployError(f"未找到命令: {cmd[0]}，请先安装")
        except subprocess.TimeoutExpired:
            raise DeployError(f"命令超时（{timeout}s）: {' '.join(cmd)}")
        if check and cp.returncode != 0:
            detail = ""
            if capture:
                detail = (cp.stderr or cp.stdout or "").strip()
            raise DeployError(f"命令失败（{cp.returncode}）: {' '.join(cmd)}" + (f"\n{detail}" if detail else ""))
        return cp

    def _tail_logs(self, service: str) -> None:
        try:
            self.run(self._compose() + ["logs", "--tail", "30", service], check=False, capture=False)
        except DeployError:
            pass

    # ---------------------------------------------------------------- docker 查询
    def compose_ps_id(self, service: str, include_stopped: bool = True) -> str:
        args = ["ps"] + (["-a"] if include_stopped else []) + ["-q", service]
        cp = self.run(self._compose() + args, check=False, capture=True)
        if cp.returncode != 0:
            return ""
        lines = [line.strip() for line in (cp.stdout or "").splitlines() if line.strip()]
        return lines[0] if lines else ""

    def service_state(self, service: str) -> Optional[ServiceState]:
        cid = self.compose_ps_id(service)
        if not cid:
            return None
        cp = self.run(["docker", "inspect", cid], check=False, capture=True)
        if cp.returncode != 0:
            return None
        try:
            data = json.loads(cp.stdout)[0]
        except (ValueError, IndexError, TypeError):
            return None
        state = data.get("State") or {}
        config = data.get("Config") or {}
        labels = config.get("Labels") or {}
        return ServiceState(
            running=bool(state.get("Running")),
            image=config.get("Image", ""),
            image_id=data.get("Image", ""),
            config_hash=labels.get("com.docker.compose.config-hash", ""),
        )

    def compose_config_hashes(self, services: List[str]) -> Dict[str, str]:
        cp = self.run(self._compose() + ["config", "--hash=*"], check=False, capture=True)
        if cp.returncode != 0:
            log_warn("docker compose config --hash 不可用，本次只按镜像判定变更（配置漂移需 --force）")
            return {}
        out: Dict[str, str] = {}
        for line in (cp.stdout or "").splitlines():
            parts = line.split()
            if len(parts) >= 2:
                out[parts[0]] = parts[1]
        return out

    def image_exists(self, image: str) -> bool:
        if not image:
            return False
        return self.run(["docker", "image", "inspect", image], check=False, capture=True).returncode == 0

    def image_id(self, image: str) -> str:
        cp = self.run(["docker", "image", "inspect", "-f", "{{.Id}}", image], check=False, capture=True)
        return (cp.stdout or "").strip() if cp.returncode == 0 else ""

    def desired_app_images(self) -> Dict[str, str]:
        images = {
            "backend": self.backend_ref,
            "frontend": self.frontend_ref,
        }
        if self.libreoffice_enabled:
            images["libreoffice"] = self.libreoffice_ref
        return images

    def desired_app_image_ids(self) -> Dict[str, str]:
        return {name: self.image_id(image) for name, image in self.desired_app_images().items() if image}

    # ---------------------------------------------------------------- 预检查
    def preflight(self) -> None:
        log_info(">>> 预部署检查...")
        for cmd in ("docker", "curl"):
            if not shutil.which(cmd):
                raise DeployError(f"未找到命令: {cmd}，请先安装")
        self.run(["docker", "compose", "version"], capture=True)
        if not self.deploy_path.is_dir():
            raise DeployError(f"部署目录不存在: {self.deploy_path}（先运行 deploy/setup-server.sh）")
        if not self.compose_file.is_file():
            raise DeployError(f"未找到 compose 文件: {self.compose_file}")
        if self.mode == "local" and not self.env_file.is_file():
            raise DeployError(f"未找到环境变量文件: {self.env_file}")
        if self.mode != "local":
            free = shutil.disk_usage(self.deploy_path).free
            if free < 2 * 1024 ** 3:
                raise DeployError(f"磁盘空间不足（{free // (1024 ** 3)}GB < 2GB）")
            log_ok(f"预检查通过（磁盘 {free // (1024 ** 3)}GB 可用）")
        else:
            log_ok("预检查通过")

    def cleanup_conflicting_containers(self) -> None:
        cp = self.run(["docker", "ps", "-a", "--format", "{{.Names}}"], check=False, capture=True)
        if cp.returncode != 0:
            return
        names = {line.strip() for line in (cp.stdout or "").splitlines()}
        if not names & set(KNOWN_CONTAINERS):
            return
        # 本项目管理的容器交给 compose；读不到项目容器时宁可不清理，也不误删。
        ps = self.run(self._compose() + ["ps", "-a", "-q"], check=False, capture=True)
        if ps.returncode != 0:
            log_warn("无法读取本项目容器列表，跳过同名残留清理")
            return
        managed = {line.strip() for line in (ps.stdout or "").splitlines() if line.strip()}
        for name in names & set(KNOWN_CONTAINERS):
            # docker ps -aq 默认给 12 位短 ID，compose ps -q 给 64 位完整 ID；
            # 直接比较字符串会把本项目容器误判成「另一个 compose 项目」（2026-10-27 实测）。
            cid = self.run(["docker", "ps", "-aq", "--filter", f"name=^/{name}$"],
                           check=False, capture=True).stdout.strip()
            if not cid:
                continue
            full_id = self.run(["docker", "inspect", "-f", "{{.Id}}", cid],
                               check=False, capture=True).stdout.strip()
            if not full_id or full_id in managed:
                continue
            labels = self.run(["docker", "inspect", "-f", "{{json .Config.Labels}}", cid],
                              check=False, capture=True)
            # 只清「不属于任何 compose 项目」的手工残留；别删别的 compose 栈的容器。
            if labels.returncode == 0 and "com.docker.compose.project" in (labels.stdout or ""):
                log_warn(f"同名容器 {name} 由另一个 compose 项目管理，保留（若 up 报冲突请人工处理）")
                continue
            log_warn(f"移除同名手工残留容器: {name}")
            self.run(["docker", "rm", "-f", name], check=False, capture=True)

    # ---------------------------------------------------------------- .env
    def render_env_file(self) -> str:
        lines: List[str] = [
            "# 由 deploy/deploy.py 自动生成 — 请勿手动编辑",
            "APP_ENV=production",
            "PORT=8080",
        ]

        def add(name: str, value: str) -> None:
            lines.append(f"{name}={value}")

        add("DATABASE_URL", env_val(getenv("DATABASE_URL")))
        add("DB_USER", env_val(getenv("DB_USER", "forklift")))
        add("DB_PASSWORD", env_val(self.db_password))

        add("SMTP_HOST", env_val(getenv("SMTP_HOST")))
        add("SMTP_PORT", getenv("SMTP_PORT", "465"))
        add("SMTP_USERNAME", env_val(getenv("SMTP_USERNAME") or getenv("SMTP_FROM")))
        add("SMTP_PASSWORD", env_val(getenv("SMTP_PASSWORD")))
        add("SMTP_FROM", env_val(getenv("SMTP_FROM")))
        add("SMTP_FROM_NAME", env_val(getenv("SMTP_FROM_NAME", "和润天下")))
        add("TENCENT_SMS_SECRET_ID", env_val(getenv("TENCENT_SMS_SECRET_ID")))
        add("TENCENT_SMS_SECRET_KEY", env_val(getenv("TENCENT_SMS_SECRET_KEY")))
        add("TENCENT_SMS_SDK_APP_ID", env_val(getenv("TENCENT_SMS_SDK_APP_ID")))
        add("TENCENT_SMS_SIGN_NAME", env_val(getenv("TENCENT_SMS_SIGN_NAME")))
        for name in (
            "TENCENT_SMS_TEMPLATE_REGISTER",
            "TENCENT_SMS_TEMPLATE_LOGIN",
            "TENCENT_SMS_TEMPLATE_PASSWORD",
            "TENCENT_SMS_TEMPLATE_BIND_PHONE",
        ):
            add(name, getenv(name))
        add("TENCENT_SMS_REGION", getenv("TENCENT_SMS_REGION", "ap-guangzhou"))
        add("WECHAT_MINI_PROGRAM_APP_ID", env_val(getenv("WECHAT_MINI_PROGRAM_APP_ID")))
        add("WECHAT_MINI_PROGRAM_APP_SECRET", env_val(getenv("WECHAT_MINI_PROGRAM_APP_SECRET")))
        add("WECHAT_OPEN_PLATFORM_APP_ID", env_val(getenv("WECHAT_OPEN_PLATFORM_APP_ID")))
        add("WECHAT_OPEN_PLATFORM_APP_SECRET", env_val(getenv("WECHAT_OPEN_PLATFORM_APP_SECRET")))

        add("SECRET_KEY", env_val(getenv("SECRET_KEY")))
        add("JWT_SECRET_KEY", env_val(getenv("JWT_SECRET_KEY")))
        add("JWT_EXPIRES_HOURS", getenv("JWT_EXPIRES_HOURS", "2"))
        add("JWT_REFRESH_EXPIRES_DAYS", getenv("JWT_REFRESH_EXPIRES_DAYS", "7"))
        add("AUTH_COOKIE_NAME", env_val(getenv("AUTH_COOKIE_NAME", "hrwai_token")))
        add("AUTH_COOKIE_DOMAIN", env_val(getenv("AUTH_COOKIE_DOMAIN")))
        add("AUTH_COOKIE_SECURE", getenv("AUTH_COOKIE_SECURE", "true"))

        add("ADMIN_DEFAULT_PASSWORD", env_val(getenv("ADMIN_DEFAULT_PASSWORD")))
        add("TUTOR_DEFAULT_PASSWORD", env_val(getenv("TUTOR_DEFAULT_PASSWORD")))
        add("STUDENT_DEFAULT_PASSWORD", env_val(getenv("STUDENT_DEFAULT_PASSWORD")))
        add("CORS_ORIGINS", env_val(getenv("CORS_ORIGINS")))

        add("BACKEND_IMAGE", self.backend_ref or getenv("BACKEND_IMAGE", "forklift-backend:latest"))
        add("FRONTEND_IMAGE", self.frontend_ref or getenv("FRONTEND_IMAGE", "forklift-frontend:latest"))
        add("LIBREOFFICE_IMAGE", self.libreoffice_ref or getenv("LIBREOFFICE_IMAGE", "forklift-libreoffice:latest"))
        add("DOMAIN", env_val(getenv("DOMAIN", "localhost")))
        add("TLS_MODE", getenv("TLS_MODE", "auto"))
        add("CADDY_ACME_EMAIL", getenv("CADDY_ACME_EMAIL"))
        add("UPLOAD_FOLDER", "/data/uploads")
        add("VOLUME_MOUNT_PATH", "/data")
        add("MAX_CONTENT_LENGTH_MB", "250")
        add("VALUATION_PDF_OUTPUT_DIR", "/data/reports")
        add("STORAGE_DRIVER", getenv("STORAGE_DRIVER", "local"))
        add("R2_ENDPOINT", env_val(getenv("R2_ENDPOINT")))
        add("R2_ACCOUNT_ID", env_val(getenv("R2_ACCOUNT_ID")))
        add("R2_ACCESS_KEY_ID", env_val(getenv("R2_ACCESS_KEY_ID")))
        add("R2_SECRET_ACCESS_KEY", env_val(getenv("R2_SECRET_ACCESS_KEY")))
        add("R2_BUCKET", getenv("R2_BUCKET"))
        add("R2_PUBLIC_DOMAIN", env_val(getenv("R2_PUBLIC_DOMAIN")))
        add("EDGE_ORIGIN_TOKEN", env_val(getenv("EDGE_ORIGIN_TOKEN")))
        add("BACKEND_HOST_PORT", getenv("BACKEND_HOST_PORT", "8080"))
        add("PG_VOLUME", getenv("PG_VOLUME", "pgdata-prod"))
        add("UPLOADS_VOLUME", getenv("UPLOADS_VOLUME", "uploads-data"))
        add("REPORTS_VOLUME", getenv("REPORTS_VOLUME", "reports-data"))
        add("LOGS_VOLUME", getenv("LOGS_VOLUME", "logs-data"))
        project = getenv("COMPOSE_PROJECT_NAME")
        if project:
            add("COMPOSE_PROJECT_NAME", project)
        return "\n".join(lines) + "\n"

    def write_env_file(self) -> bool:
        content = self.render_env_file()
        path = self.env_file
        try:
            current = path.read_text(encoding="utf-8")
        except OSError:
            current = None
        if current == content:
            log_ok(".env 未变化，跳过写入")
            return False
        path.parent.mkdir(parents=True, exist_ok=True)
        tmp = path.with_name(f".env.tmp.{os.getpid()}")
        tmp.write_text(content, encoding="utf-8")
        os.chmod(tmp, 0o600)
        os.replace(tmp, path)
        log_ok(f".env 已生成（{len(content.splitlines())} 行）")
        return True

    # ---------------------------------------------------------------- 镜像
    def login_registry(self) -> None:
        log_info(">>> 登录镜像注册表...")
        if self.github_token and self.registry == "ghcr.io":
            cp = subprocess.run(
                ["docker", "login", self.registry, "-u", "deploy", "--password-stdin"],
                input=self.github_token, text=True, capture_output=True,
            )
            if cp.returncode == 0:
                log_ok(f"已登录 {self.registry}")
            else:
                log_warn("注册表登录失败，依赖本地缓存镜像")
        elif getenv("REGISTRY_USERNAME") and getenv("REGISTRY_PASSWORD"):
            cp = subprocess.run(
                ["docker", "login", self.registry, "-u", getenv("REGISTRY_USERNAME"), "--password-stdin"],
                input=getenv("REGISTRY_PASSWORD"), text=True, capture_output=True,
            )
            if cp.returncode == 0:
                log_ok(f"已登录 {self.registry}")
            else:
                log_warn("注册表登录失败，依赖本地缓存镜像")
        else:
            log_warn("未配置注册表凭据，依赖本地缓存镜像")

    def _start_registry_proxy(self) -> None:
        self.run(["docker", "rm", "-f", "ghcr-proxy"], check=False, capture=True)
        log_info("启动 ghcr pull-through 缓存容器 (registry:2)...")
        self.run(["docker", "pull", "registry:2"], check=False, capture=True)
        args = [
            "docker", "run", "-d", "--name", "ghcr-proxy", "--restart", "unless-stopped",
            "-p", f"{self.registry_proxy_bind}:5000:5000",
            "-e", "REGISTRY_PROXY_REMOTEURL=https://ghcr.io",
            "-v", "ghcr-cache:/var/lib/registry",
        ]
        token_file = Path(getenv("GHCR_PULL_TOKEN_FILE", str(self.deploy_path / ".ghcr-pull-token")))
        if token_file.is_file():
            try:
                token = token_file.read_text(encoding="utf-8").strip()
            except OSError:
                token = ""
            if token:
                args += ["-e", "REGISTRY_PROXY_USERNAME=oauth2", "-e", f"REGISTRY_PROXY_PASSWORD={token}"]
        args.append("registry:2")
        self.run(args, check=False, capture=True)
        if self.run(["docker", "ps", "--format", "{{.Names}}"], check=False, capture=True).stdout.find("ghcr-proxy") < 0:
            self.run(["docker", "start", "ghcr-proxy"], check=False, capture=True)

    def _proxy_reachable(self) -> bool:
        try:
            cp = self.run(
                ["curl", "-s", "-o", "/dev/null", f"http://{self.registry_proxy}/v2/"],
                check=False, capture=True, timeout=5,
            )
        except DeployError:
            return False
        return cp.returncode == 0

    def ensure_registry_proxy(self) -> None:
        if not self.registry_proxy:
            return
        log_info(f">>> 检查镜像加速代理: {self.registry_proxy} (bind={self.registry_proxy_bind}) ...")
        host = self.registry_proxy.split(":", 1)[0]
        is_local = host in ("127.0.0.1", "localhost")
        if not is_local:
            cp = self.run(["ip", "-4", "addr", "show"], check=False, capture=True)
            is_local = cp.returncode == 0 and f"inet {host}/" in (cp.stdout or "")
        if is_local:
            cache_dir = Path("/var/lib/docker/volumes/ghcr-cache/_data")
            if cache_dir.is_dir():
                try:
                    size_mb = sum(p.stat().st_size for p in cache_dir.rglob("*") if p.is_file()) // (1024 * 1024)
                except OSError:
                    size_mb = 0
                if size_mb > self.cache_limit_mb:
                    log_warn(f"ghcr-cache 缓存 {size_mb}MB 超过 {self.cache_limit_mb}MB 阈值，清空缓存卷")
                    self.run(["docker", "rm", "-f", "ghcr-proxy"], check=False, capture=True)
                    self.run(["docker", "volume", "rm", "ghcr-cache"], check=False, capture=True)
            exists = "ghcr-proxy" in (self.run(["docker", "ps", "-a", "--format", "{{.Names}}"], check=False, capture=True).stdout or "")
            running = "ghcr-proxy" in (self.run(["docker", "ps", "--format", "{{.Names}}"], check=False, capture=True).stdout or "")
            if not exists:
                self._start_registry_proxy()
            elif not running:
                self.run(["docker", "start", "ghcr-proxy"], check=False, capture=True)

        for _ in range(10):
            if self._proxy_reachable():
                log_ok(f"镜像加速代理就绪: {self.registry_proxy}")
                return
            time.sleep(1)
        log_warn(f"镜像加速代理未就绪（{self.registry_proxy}），本次改走国内镜像源 {self.registry_mirror or '（未配置）'}")
        self.backend_ref = self.backend_ref_orig
        self.frontend_ref = self.frontend_ref_orig
        self.libreoffice_ref = self.libreoffice_ref_orig
        self._rewrite_to_mirror()

    def ensure_proxy_healthy(self) -> bool:
        if not self.registry_proxy:
            return True
        if self._proxy_reachable():
            return True
        log_warn("镜像加速代理探测超时，重启 ghcr-proxy ...")
        self.run(["docker", "restart", "ghcr-proxy"], check=False, capture=True)
        time.sleep(3)
        if self._proxy_reachable():
            log_ok("镜像加速代理重启成功")
            return True
        log_warn("镜像加速代理重启后仍不可用，删除重建（缓存卷保留）...")
        self._start_registry_proxy()
        time.sleep(3)
        if self._proxy_reachable():
            log_ok("镜像加速代理重建成功")
            return True
        log_error("镜像加速代理重建后仍不可用，本次拉取走镜像源/直连回退链路")
        return False

    def _pull_with_watchdog(self, image: str) -> bool:
        idle_timeout = env_int("DOCKER_PULL_IDLE_TIMEOUT", 180)
        first_timeout = env_int("DOCKER_PULL_FIRST_TIMEOUT", 60)
        total_timeout = env_int("DOCKER_PULL_TOTAL_TIMEOUT", 2700)
        fd, log_path = tempfile.mkstemp(prefix="pull-", suffix=".log")
        os.close(fd)
        log_file = Path(log_path)
        try:
            with log_file.open("wb") as fh:
                proc = subprocess.Popen(["docker", "pull", image], stdout=fh, stderr=subprocess.STDOUT)
            start = time.time()
            last_change = start
            last_size = 0
            while proc.poll() is None:
                time.sleep(10)
                now = time.time()
                if now - start >= total_timeout:
                    log_warn(f"拉取总耗时超 {total_timeout}s，终止（总超时兜底）")
                    proc.kill()
                    proc.wait()
                    return False
                try:
                    size = log_file.stat().st_size
                except OSError:
                    size = 0
                if size != last_size:
                    try:
                        with log_file.open("rb") as fh:
                            fh.seek(last_size)
                            chunk = fh.read()
                        if chunk:
                            sys.stderr.write(chunk.decode("utf-8", "replace"))
                            sys.stderr.flush()
                    except OSError:
                        pass
                    last_size = size
                    last_change = now
                elif last_size == 0 and now - start >= first_timeout:
                    log_warn(f"拉取 {first_timeout}s 零输出（上游回源疑似卡死），终止并走回退链路")
                    proc.kill()
                    proc.wait()
                    return False
                elif last_size > 0 and now - last_change >= idle_timeout:
                    log_warn(f"拉取 {idle_timeout}s 无新输出，判定挂起并终止（进度看门狗）")
                    proc.kill()
                    proc.wait()
                    return False
            rc = proc.wait()
            try:
                with log_file.open("rb") as fh:
                    fh.seek(last_size)
                    chunk = fh.read()
                if chunk:
                    sys.stderr.write(chunk.decode("utf-8", "replace"))
                    sys.stderr.flush()
            except OSError:
                pass
            return rc == 0
        finally:
            try:
                log_file.unlink()
            except OSError:
                pass

    def _pull_one(self, name: str, image: str) -> bool:
        core = ""
        for prefix, registry in (
            (self.registry_proxy, self.registry_proxy),
            (self.registry_mirror, self.registry_mirror),
            (self.registry, self.registry),
        ):
            if prefix and image.startswith(prefix + "/"):
                core = image[len(prefix) + 1:]
                break

        # 直连优先（#635 实测：ghcr CDN 晚高峰时代理/镜像源更容易卡死）。
        if core and self.github_token:
            ghcr_ref = f"{self.registry}/{core}"
            if image == ghcr_ref:
                log_info(f"拉取{name}镜像（直连）: {image}")
                if self._pull_with_watchdog(image):
                    log_ok(f"{name}镜像: {image}")
                    return True
            else:
                log_info(f"拉取{name}镜像（直连优先）: {ghcr_ref}")
                subprocess.run(
                    ["docker", "login", self.registry, "-u", "oauth2", "--password-stdin"],
                    input=self.github_token, text=True, capture_output=True,
                )
                if self._pull_with_watchdog(ghcr_ref):
                    self.run(["docker", "tag", ghcr_ref, image], check=False, capture=True)
                    self.run(["docker", "rmi", ghcr_ref], check=False, capture=True)
                    log_ok(f"直连拉取成功并补 tag: {image}")
                    return True
                log_warn("直连拉取失败，转代理路径重试...")

        retries = 2
        for attempt in range(1, retries + 1):
            log_info(f"拉取{name}镜像（代理尝试 {attempt}/{retries}）: {image}")
            if self._pull_with_watchdog(image):
                log_ok(f"{name}镜像: {image}")
                return True
            if attempt < retries:
                log_warn("拉取失败，10 秒后重试...")
                time.sleep(10)
                self.ensure_proxy_healthy()

        if core and self.registry == "ghcr.io" and self.registry_mirror and self.github_token and image != f"{self.registry_mirror}/{core}":
            mirror_ref = f"{self.registry_mirror}/{core}"
            log_warn(f"代理拉取失败，回退国内镜像源 {self.registry_mirror} 认证拉取: {mirror_ref}")
            subprocess.run(
                ["docker", "login", self.registry_mirror, "-u", "oauth2", "--password-stdin"],
                input=self.github_token, text=True, capture_output=True,
            )
            if self._pull_with_watchdog(mirror_ref):
                self.run(["docker", "tag", mirror_ref, image], check=False, capture=True)
                self.run(["docker", "rmi", mirror_ref], check=False, capture=True)
                log_ok(f"镜像源拉取成功并补 tag: {image}")
                return True

        log_error(f"{name}镜像拉取失败（直连优先 + 代理 {retries} 次 + 镜像源回退）: {image}")
        return False

    def pull_images(self) -> None:
        log_info(">>> 拉取最新镜像...")
        self.ensure_proxy_healthy()
        targets = [("后端", self.backend_ref), ("前端", self.frontend_ref)]
        if self.libreoffice_enabled:
            targets.append(("LibreOffice sidecar", self.libreoffice_ref))
        for name, image in targets:
            if not image:
                continue
            if self.image_exists(image):
                log_ok(f"{name}镜像已缓存，跳过拉取: {image}")
                continue
            if not self._pull_one(name, image):
                raise DeployError(f"{name}镜像拉取失败: {image}")
        for base in ("postgres:18-alpine",):
            if self.image_exists(base):
                log_ok(f"基础镜像已缓存: {base}")
                continue
            log_info(f"拉取基础镜像: {base}")
            # 走 watchdog 而不是裸 docker pull：镜像源挂住时裸拉没有任何超时，
            # 整个部署会被拖到外层 timeout 兜底（2026-10-29 testing 冒烟因此挂起 35 分钟）。
            if not self._pull_with_watchdog(base):
                raise DeployError(f"基础镜像拉取失败: {base}")

    # ---------------------------------------------------------------- 数据服务
    def plan_data(self) -> List[Decision]:
        hashes = self.compose_config_hashes(list(DATA_SERVICES))
        states = {name: self.service_state(name) for name in DATA_SERVICES}
        return plan_decisions(
            {name: "" for name in DATA_SERVICES},
            states,
            hashes,
            services=list(DATA_SERVICES),
            force_data=self.opts.force_data_services,
        )

    def apply_data(self, decisions: List[Decision]) -> None:
        blocked = [d for d in decisions if d.action == "blocked"]
        if blocked:
            detail = "；".join(f"{d.service}: {d.reason}" for d in blocked)
            raise DeployError(
                f"数据服务配置发生变更，拒绝自动重建（{detail}）。\n"
                "  确认卷/配置变更是有意为之后，显式加 --force-data-services 重跑。"
            )
        targets = [d.service for d in decisions if d.changes_container]
        if not targets:
            log_ok("数据服务 postgres 已在运行，跳过")
            return
        for d in decisions:
            if d.changes_container:
                log_info(f">>> {d.service}: {d.reason}")
        self.run(self._compose() + ["up", "-d", "--no-deps", *targets])

    def wait_postgres(self) -> bool:
        log_info(">>> 等待数据库就绪...")
        for _ in range(15):
            cid = self.compose_ps_id("postgres")
            if cid:
                cp = self.run(
                    ["docker", "exec", cid, "pg_isready", "-U", self.db_user, "-d", self.db_name],
                    check=False, capture=True,
                )
                if cp.returncode == 0:
                    log_ok("数据库就绪")
                    return True
            time.sleep(2)
        log_warn("数据库在超时时间内未就绪，后续依赖数据库的步骤可能失败")
        return False

    # ---------------------------------------------------------------- 备份
    def capture_previous_images(self) -> None:
        for service, attr in (("backend", "previous_backend_image"), ("frontend", "previous_frontend_image")):
            cid = self.compose_ps_id(service, include_stopped=False)
            value = "unknown"
            if cid:
                cp = self.run(["docker", "inspect", "-f", "{{.Config.Image}}", cid], check=False, capture=True)
                value = (cp.stdout or "").strip() or "unknown"
            setattr(self, attr, value)
        if self.mode != "local":
            self.backup_dir.mkdir(parents=True, exist_ok=True)
            (self.backup_dir / "last-version.txt").write_text(
                f"PREVIOUS_BACKEND_IMAGE={self.previous_backend_image}\n"
                f"PREVIOUS_FRONTEND_IMAGE={self.previous_frontend_image}\n",
                encoding="utf-8",
            )

    def _backup_once(self) -> None:
        try:
            self._create_backup()
        except Exception as exc:  # noqa: BLE001 — 备份失败不应阻断部署（与旧脚本一致）
            self.backup_error = str(exc)
            log_warn(f"备份过程异常（继续部署）: {exc}")

    def _create_backup(self) -> None:
        log_info(">>> 创建部署前备份...")
        stamp = time.strftime("%Y%m%d_%H%M%S")
        backup_file = self.backup_dir / f"backup_{stamp}.txt"
        db_file = self.backup_dir / f"db_backup_{stamp}.sql.gz"
        self.backup_dir.mkdir(parents=True, exist_ok=True)

        pg_id = self.compose_ps_id("postgres")
        if pg_id:
            log_info("备份数据库（pg_dump + gzip）...")
            proc = subprocess.Popen(
                ["docker", "exec", pg_id, "pg_dump", "-U", self.db_user, "-d", self.db_name],
                stdout=subprocess.PIPE, stderr=subprocess.DEVNULL,
            )
            ok = False
            try:
                with gzip.open(db_file, "wb", compresslevel=9) as gz:
                    assert proc.stdout is not None
                    shutil.copyfileobj(proc.stdout, gz)
                ok = proc.wait() == 0
            finally:
                if proc.poll() is None:
                    proc.kill()
                    proc.wait()
            if ok:
                size = db_file.stat().st_size // 1024
                log_ok(f"数据库备份完成: {db_file} ({size}KB)")
            else:
                self.backup_error = "数据库备份失败"
                log_warn("数据库备份失败，继续部署（迁移失败将无法回滚数据）")
                db_file.unlink(missing_ok=True)
        else:
            log_warn("数据库未运行，跳过数据库备份")

        compose = self._compose()
        ps_out = self.run(compose + ["ps"], check=False, capture=True).stdout or ""
        images_out = self.run(compose + ["images"], check=False, capture=True).stdout or ""
        env_dump = ""
        try:
            for line in self.env_file.read_text(encoding="utf-8").splitlines():
                if re.search(r"(DATABASE_URL|SECRET_KEY|JWT_SECRET_KEY|PASSWORD|API_KEY|R2_SECRET|R2_ACCESS_KEY|TENCENT_SMS_SECRET|EDGE_ORIGIN_TOKEN)", line):
                    continue
                env_dump += line + "\n"
        except OSError:
            pass
        backup_file.write_text(
            f"=== 备份时间: {time.strftime('%Y-%m-%d %H:%M:%S')} ===\n"
            f"=== Git 提交: {getenv('IMAGE_TAG', 'unknown')} ===\n"
            f"=== 镜像标签: backend={self.tag_backend} frontend={self.tag_frontend} libreoffice={self.tag_libreoffice} ===\n\n"
            f"--- 运行中的容器 ---\n{ps_out}\n--- 当前镜像 ---\n{images_out}\n"
            f"--- 备份 .env（已过滤敏感项） ---\n{env_dump}\n"
            + (f"--- 数据库备份 ---\n文件: {db_file}\n" if db_file.exists() else ""),
            encoding="utf-8",
        )

        # 异地同步（仅配置了 BACKUP_REMOTE_HOST 时）
        remote = getenv("BACKUP_REMOTE_HOST")
        if remote and db_file.exists():
            remote_dir = getenv("BACKUP_REMOTE_DIR", "/opt/forklift-backups")
            key = getenv("BACKUP_REMOTE_KEY", "/root/.ssh/pve01-sync")
            log_info(f"同步数据库备份到 {remote}...")
            self.run(["ssh", "-i", key, "-o", "StrictHostKeyChecking=no", "-o", "ConnectTimeout=10",
                      remote, f"mkdir -p {remote_dir}"], check=False, capture=True)
            cp = self.run(["scp", "-i", key, "-o", "StrictHostKeyChecking=no", "-o", "ConnectTimeout=10",
                           str(db_file), f"{remote}:{remote_dir}/"], check=False, capture=True)
            if cp.returncode == 0:
                log_ok(f"异地备份完成: {remote}:{remote_dir}/{db_file.name}")
                self.run(["ssh", "-i", key, "-o", "StrictHostKeyChecking=no", remote,
                          f"ls -t {remote_dir}/db_backup_*.sql.gz 2>/dev/null | tail -n +11 | xargs rm -f"],
                         check=False, capture=True)
            else:
                log_warn(f"异地备份失败（继续部署）: {(cp.stderr or '').strip()}")

        # 保留最近 10 份
        for pattern in ("db_backup_*.sql.gz", "backup_*.txt"):
            files = sorted(self.backup_dir.glob(pattern), key=lambda p: p.stat().st_mtime, reverse=True)
            for old in files[10:]:
                try:
                    old.unlink()
                except OSError:
                    pass
        log_ok(f"备份完成: {backup_file}")

    # ---------------------------------------------------------------- 迁移
    def _image_max_migration(self, image: str) -> Optional[int]:
        if not image or not self.image_exists(image):
            return None
        cp = self.run(
            ["docker", "run", "--rm", "--entrypoint", "sh", image, "-c",
             'ls /app/migrations/*.up.sql 2>/dev/null | grep -oE "[0-9]{6}" | sort -n | tail -1'],
            check=False, capture=True,
        )
        value = (cp.stdout or "").strip()
        return int(value) if value.isdigit() else 0

    def _db_url(self) -> str:
        user = quote(self.db_user, safe="")
        password = quote(self.db_password or "", safe="")
        return f"postgres://{user}:{password}@localhost:5432/{self.db_name}?sslmode=disable"

    def _db_version(self) -> int:
        pg_id = self.compose_ps_id("postgres")
        if not pg_id:
            return 0
        cp = self.run(
            ["docker", "exec", pg_id, "psql", "-U", self.db_user, "-d", self.db_name, "-tAc",
             "SELECT COALESCE(version,0) FROM schema_migrations LIMIT 1;"],
            check=False, capture=True,
        )
        value = (cp.stdout or "").strip()
        return int(value) if value.isdigit() else 0

    def preflight_migration_check(self, image: str) -> bool:
        if not image:
            log_warn("未指定镜像，跳过迁移预检")
            return True
        log_info(f">>> 迁移兼容性预检: {image}")
        img_max = self._image_max_migration(image)
        if img_max is None:
            log_warn(f"本地无镜像 {image}，无法读取迁移版本，跳过预检")
            return True
        db_ver = self._db_version()
        log_info(f"镜像最大迁移版本={img_max}, 数据库当前版本={db_ver}")
        if db_ver > img_max:
            log_error(f"镜像迁移版本({img_max})落后于数据库({db_ver})!")
            log_error("该镜像启动后会因缺少迁移文件而崩溃循环（no migration found for version ...）。")
            return False
        log_ok("迁移兼容性预检通过")
        return True

    def migration_needed(self, image: str) -> bool:
        if self.skip_migration:
            return False
        img_max = self._image_max_migration(image)
        if img_max is None:
            return True
        return self._db_version() < img_max

    def fix_dirty_state(self) -> None:
        log_info(">>> 检查数据库迁移状态...")
        pg_id = self.compose_ps_id("postgres")
        if not pg_id:
            log_warn("PostgreSQL 容器未运行，跳过 dirty 检查")
            return
        cur_ver = self._db_version()
        cp = self.run(
            ["docker", "exec", pg_id, "psql", "-U", self.db_user, "-d", self.db_name, "-tAc",
             "SELECT COALESCE(dirty,false) FROM schema_migrations LIMIT 1;"],
            check=False, capture=True,
        )
        dirty = (cp.stdout or "").strip().lower()
        if dirty not in ("t", "true"):
            log_ok(f"数据库迁移状态正常 (version={cur_ver}, dirty=false)")
            return
        log_warn(f"数据库处于 dirty 状态 (version={cur_ver}, dirty=true)")
        force_ver = max(cur_ver - 1, 0)
        log_info(f"自动修复：执行 migrate force {force_ver} 清除 dirty 标志")
        url = self._db_url()
        cp = self.run(
            ["docker", "run", "--rm", "--network", f"container:{pg_id}",
             "-e", f"DATABASE_URL={url}", self.backend_ref, "/app/bin/migrate", "force", str(force_ver)],
            check=False,
        )
        if cp.returncode != 0:
            raise DeployError(
                "自动 force 失败，需手动修复：\n"
                f"  docker exec {pg_id} psql -U {self.db_user} -d {self.db_name} "
                f'-c "UPDATE schema_migrations SET version={force_ver}, dirty=false;"'
            )
        log_ok(f"已清除 dirty 标志（force 到版本 {force_ver}）")

    def run_migration(self) -> bool:
        if self.skip_migration:
            log_info(">>> 跳过数据库迁移（SKIP_MIGRATION / --skip-migrate）")
            return True
        log_info(">>> 执行数据库迁移...")
        pg_id = self.compose_ps_id("postgres")
        if not pg_id:
            raise DeployError("PostgreSQL 容器未运行，无法迁移")
        url = self._db_url()
        log_info(f"运行迁移: {self.backend_ref} /app/bin/migrate up")
        cp = self.run(
            ["docker", "run", "--rm", "--network", f"container:{pg_id}",
             "-e", f"DATABASE_URL={url}", self.backend_ref, "/app/bin/migrate", "up"],
            check=False,
        )
        if cp.returncode == 0:
            log_ok("数据库迁移完成")
            return True
        if migration_failure_action(self.allow_migration_failure) == "continue":
            log_warn("自动迁移失败，但 ALLOW_MIGRATION_FAILURE=1 已显式声明——带已知风险继续部署")
            log_warn("待办（值班人）：cd backend && go run ./cmd/migrate up")
            return True
        log_error("自动迁移失败，中止部署（避免新代码跑在旧 schema 上）")
        raise DeployError("数据库迁移失败。线上事故确需继续时，显式声明 ALLOW_MIGRATION_FAILURE=1 后重跑。")

    # ---------------------------------------------------------------- 应用服务
    def plan_app(self) -> List[Decision]:
        services = [s for s in APP_SERVICES if s != "libreoffice" or self.libreoffice_enabled]
        desired = self.desired_app_images()
        states = {name: self.service_state(name) for name in services}
        hashes = self.compose_config_hashes(services)
        ids = self.desired_app_image_ids()
        return plan_decisions(
            desired, states, hashes, ids,
            force=self.opts.force,
            services=services,
        )

    def apply_app(self, decisions: List[Decision]) -> None:
        changes = [d for d in decisions if d.changes_container]
        if not changes:
            log_ok("应用服务无变更，跳过重建（backend / frontend 保持运行）")
            return
        for d in decisions:
            label = {"skip": "跳过", "start": "启动", "create": "创建", "recreate": "重建", "blocked": "阻止"}[d.action]
            log_info(f">>> {d.service}: {label} — {d.reason}")
        for d in changes:
            # 不传 --wait：compose 的 wait 只看容器 healthcheck，backend 的 start_period
            # 会在 testing 的 30s 预算下误报「application not healthy after 30s」
            # （2026-10-27 实测；容器其实已在正常启动）。真正的就绪判据交给下方 health_check 的 HTTP 轮询。
            #
            # 不能带 --no-deps：compose 的 config-hash 包含 depends_on，而 --no-deps 创建出的容器
            # 标签哈希与 compose config --hash 永远不同 —— backend / frontend 因此每次都被判
            # 「compose 配置变更」重建（2026-10-27 实测：no-deps=1e85… vs config/normal=7f1f…）。
            # 去掉后 up 与 config 用同一份哈希；数据服务仍由 plan_data 的 blocked 保护，
            # 且 up <app> 不会重建未变化的依赖（--force-recreate 也只作用于指定服务）。
            args = ["up", "-d"]
            if self.opts.force:
                args += ["--force-recreate"]
            if self.opts.remove_orphans:
                args += ["--remove-orphans"]
            args.append(d.service)
            cp = self.run(self._compose() + args, check=False)
            if cp.returncode != 0:
                self._tail_logs(d.service)
                raise DeployError(f"{d.service} 启动失败")
            self.applied.append(d.service)

    # ---------------------------------------------------------------- 健康检查
    def health_check(self) -> None:
        log_info(">>> 后端健康检查 (localhost:8080/api/health)...")
        backend_ok = False
        for attempt in range(1, self.health_retries + 1):
            cid = self.compose_ps_id("backend")
            if cid:
                cp = self.run(
                    self._compose() + ["exec", "-T", "backend", "wget", "-qO-", "http://localhost:8080/api/health"],
                    check=False, capture=True,
                )
                if cp.returncode == 0 and '"status":"ok"' in (cp.stdout or ""):
                    backend_ok = True
                    break
            if attempt % 5 == 0:
                log_warn(f"等待服务就绪... [{attempt}/{self.health_retries}]")
            time.sleep(self.health_interval)
        if not backend_ok:
            log_error("后端健康检查超时!")
            self._tail_logs("backend")
            self.run(self._compose() + ["ps"], check=False)
            raise DeployError("后端健康检查失败")

        log_ok("后端健康检查通过")
        log_info(">>> 前端健康检查...")
        fe_host = f"api.{getenv('DOMAIN', 'localhost')}"
        frontend_ok = False
        code = "000"
        for attempt in range(1, self.health_retries + 1):
            cp = self.run(
                ["curl", "-sk", "-o", "/dev/null", "-w", "%{http_code}", "--max-time", "5",
                 "--resolve", f"{fe_host}:443:127.0.0.1", f"https://{fe_host}/health"],
                check=False, capture=True, timeout=10,
            )
            code = (cp.stdout or "").strip() if cp.returncode == 0 else "000"
            if code == "200":
                frontend_ok = True
                break
            if attempt % 5 == 0:
                log_warn(f"等待前端就绪... ({code}) [{attempt}/{self.health_retries}]")
            time.sleep(self.health_interval)
        if not frontend_ok:
            log_error(f"前端健康检查失败 (HTTP {code})!")
            self._tail_logs("frontend")
            self.run(self._compose() + ["ps"], check=False)
            raise DeployError("前端健康检查失败")
        log_ok(f"前端健康检查通过 ({code})")

    # ---------------------------------------------------------------- 清理
    def cleanup(self) -> None:
        log_info(">>> 清理旧资源...")
        dangling = self.run(["docker", "images", "-f", "dangling=true", "-q"], check=False, capture=True)
        if (dangling.stdout or "").strip():
            self.run(["docker", "image", "prune", "-f"], check=False, capture=True)
            log_ok("已清理悬空镜像")
        self.prune_old_images()
        if self.backup_dir.is_dir():
            for old in sorted(self.backup_dir.glob("backup_*.txt"), key=lambda p: p.stat().st_mtime, reverse=True)[10:]:
                try:
                    old.unlink()
                except OSError:
                    pass
        self.run(["docker", "builder", "prune", "-f", "--filter", "until=72h"], check=False, capture=True)
        log_ok("清理完成")

    def prune_old_images(self) -> None:
        if self.keep_images <= 0:
            log_info("KEEP_IMAGES<=0，跳过旧镜像清理")
            return
        repos = [self.backend_ref, self.frontend_ref, self.libreoffice_ref,
                 self.backend_ref_orig, self.frontend_ref_orig, self.libreoffice_ref_orig]
        if self.registry_mirror:
            for orig in (self.backend_ref_orig, self.frontend_ref_orig, self.libreoffice_ref_orig):
                if orig.startswith("ghcr.io/"):
                    repos.append(f"{self.registry_mirror}/{orig[len('ghcr.io/'):]}")
        for repo in [r for r in repos if r]:
            base = _strip_tag(repo)
            cp = self.run(
                ["docker", "images", "--format", "{{.CreatedAt}}|{{.ID}}|{{.Repository}}:{{.Tag}}", base],
                check=False, capture=True,
            )
            rows = [line for line in (cp.stdout or "").splitlines() if "|<none>" not in line]
            for row in sorted(rows, reverse=True)[self.keep_images:]:
                parts = row.split("|")
                if len(parts) < 3:
                    continue
                log_info(f"删除旧镜像: {parts[2]} ({parts[1]})")
                self.run(["docker", "image", "rm", "-f", parts[1]], check=False, capture=True)
        log_ok("旧镜像清理完成")

    # ---------------------------------------------------------------- 部署主流程
    def print_plan(self, json_output: bool = False) -> None:
        data_decisions = self.plan_data()
        app_decisions = self.plan_app()
        migration = self.migration_needed(self.backend_ref)
        if json_output:
            payload = {
                "mode": self.mode,
                "deploy_path": str(self.deploy_path),
                "compose_file": str(self.compose_file),
                "images": {
                    "backend": self.backend_ref,
                    "frontend": self.frontend_ref,
                    "libreoffice": self.libreoffice_ref if self.libreoffice_enabled else "",
                },
                "migration_needed": migration,
                "decisions": [
                    {"service": d.service, "action": d.action, "reason": d.reason, "desired_image": d.desired_image}
                    for d in data_decisions + app_decisions
                ],
            }
            print(json.dumps(payload, ensure_ascii=False, indent=2))
            return
        print("")
        print(f"{'服务':<12}{'动作':<8}原因")
        print("-" * 72)
        for d in data_decisions + app_decisions:
            label = {"skip": "跳过", "start": "启动", "create": "创建", "recreate": "重建", "blocked": "阻止"}[d.action]
            print(f"{d.service:<12}{label:<8}{d.reason}")
        print("-" * 72)
        print(f"数据库迁移: {'需要' if migration else '不需要（已是最新版本）'}")
        print(f"镜像: backend={self.backend_ref or '(本地构建)'} frontend={self.frontend_ref or '(本地构建)'}")

    def _validate_compose(self) -> None:
        cp = self.run(self._compose() + ["config", "-q"], check=False, capture=True)
        if cp.returncode != 0:
            raise DeployError(f"compose 配置校验失败:\n{(cp.stderr or cp.stdout or '').strip()}")

    def deploy(self) -> None:
        self.preflight()
        # 先解析代理/镜像源，再写 .env —— 否则代理回退后 .env 里的镜像引用会与实际拉取路径不一致。
        self.login_registry()
        self.ensure_registry_proxy()
        if self.mode != "local":
            self.write_env_file()
        self._validate_compose()
        # 放在 .env 落盘之后：同名残留清理与后续 up / 变更计划必须用同一个 compose 项目名。
        self.cleanup_conflicting_containers()
        self.capture_previous_images()

        backup_thread: Optional[threading.Thread] = None
        if self.mode != "local" and not self.skip_backup:
            backup_thread = threading.Thread(target=self._backup_once, name="deploy-backup", daemon=True)
            backup_thread.start()

        try:
            self.pull_images()

            data_decisions = self.plan_data()
            self.apply_data(data_decisions)
            self.wait_postgres()

            if not self.preflight_migration_check(self.backend_ref):
                raise DeployError("迁移预检失败，终止部署（避免后端因缺少迁移文件崩溃循环）")

            # 备份必须先于任何 DB 写操作（fix_dirty / migrate）。
            if backup_thread is not None:
                backup_thread.join(timeout=self.backup_wait_timeout)
                if backup_thread.is_alive():
                    log_warn(f"备份超时（{self.backup_wait_timeout}s），继续部署（本次回滚保障可能不完整）")
                elif self.backup_error:
                    log_warn(f"备份未完整完成（继续部署）: {self.backup_error}")
                else:
                    log_ok("备份已完成")

            # dirty 状态与「是否有待跑迁移」是两件事：即使版本已最新也要清 dirty。
            self.fix_dirty_state()
            if self.migration_needed(self.backend_ref):
                self.run_migration()
            else:
                log_ok("数据库已是最新迁移版本，跳过 migrate")

            app_decisions = self.plan_app()
            blocked = [d for d in app_decisions if d.action == "blocked"]
            if blocked:
                raise DeployError("；".join(f"{d.service}: {d.reason}" for d in blocked))
            for d in app_decisions:
                label = {"skip": "跳过", "start": "启动", "create": "创建", "recreate": "重建", "blocked": "阻止"}[d.action]
                log_info(f"计划 {d.service}: {label}（{d.reason}）")
            self.apply_app(app_decisions)

            self.health_check()
        except DeployError:
            # 只有真的动过应用容器才回滚；镜像拉取/迁移阶段失败时线上版本没变，不需要回滚。
            if self.applied and (self.backup_dir / "last-version.txt").is_file():
                log_warn("部署失败，尝试回滚已变更的服务...")
                try:
                    self.do_rollback()
                except Exception as exc:  # noqa: BLE001 — 回滚失败也要保留原始失败
                    log_error(f"自动回滚失败: {exc}")
            raise

        # 清理失败只告警，不触发回滚（部署与健康检查已经成功）。
        try:
            self.cleanup()
        except Exception as exc:  # noqa: BLE001
            log_warn(f"清理旧资源失败（不影响本次部署）: {exc}")
        log_ok("部署完成!")

    # ---------------------------------------------------------------- 回滚
    def _read_last_version(self) -> Dict[str, str]:
        path = self.backup_dir / "last-version.txt"
        if not path.is_file():
            raise DeployError(f"未找到备份文件，无法回滚: {path}")
        out: Dict[str, str] = {}
        for line in path.read_text(encoding="utf-8").splitlines():
            if "=" in line:
                k, v = line.split("=", 1)
                out[k.strip()] = v.strip()
        return out

    def assert_running_image(self, service: str, expected: str) -> bool:
        cid = self.compose_ps_id(service, include_stopped=False)
        if not cid:
            log_error(f"回滚自证失败：{service} 没有运行中的容器")
            return False
        cp = self.run(["docker", "inspect", "-f", "{{.Config.Image}}", cid], check=False, capture=True)
        actual = (cp.stdout or "").strip() or "unknown"
        if not images_equal(actual, expected):
            log_error(f"回滚自证失败：{service} 运行镜像 = {actual}，期望 = {expected}")
            return False
        log_ok(f"回滚自证通过：{service} = {actual}")
        return True

    def do_rollback(self) -> None:
        log_warn(">>> 执行回滚操作...")
        previous = self._read_last_version()
        prev_backend = previous.get("PREVIOUS_BACKEND_IMAGE", "unknown")
        prev_frontend = previous.get("PREVIOUS_FRONTEND_IMAGE", "unknown")
        prev_backend_tag = rollback_tag(prev_backend)
        prev_frontend_tag = rollback_tag(prev_frontend)

        self.apply_data(self.plan_data())
        self.wait_postgres()

        # 「是否真的回滚这个服务」只看 tag 是否变化：相同 tag 不重建，避免回滚把无关容器也重启。
        cur_backend_tag = rollback_tag(self.backend_ref)
        cur_frontend_tag = rollback_tag(self.frontend_ref)
        backend_base = _strip_tag(self.backend_ref) or _strip_tag(self.backend_ref_orig)
        frontend_base = _strip_tag(self.frontend_ref) or _strip_tag(self.frontend_ref_orig)
        backend_changed = bool(prev_backend_tag and backend_base and prev_backend_tag != cur_backend_tag)
        frontend_changed = bool(prev_frontend_tag and frontend_base and prev_frontend_tag != cur_frontend_tag)

        # 后端：迁移预检不过则保持当前版本（避免回滚到会崩溃循环的镜像）。
        if backend_changed:
            candidate = f"{backend_base}:{prev_backend_tag}"
            if self.preflight_migration_check(candidate):
                self.tag_backend = prev_backend_tag
                self.backend_ref = candidate
                log_info(f"回滚后端到: {candidate}")
            else:
                backend_changed = False
                log_error(f"拒绝回滚后端到 {candidate}（迁移版本落后数据库，会崩溃循环）")
                log_error(f"保持当前后端镜像: {self.backend_ref}")
        elif prev_backend_tag:
            log_ok(f"后端已是历史版本 tag={prev_backend_tag}，无需重建")
        else:
            log_warn("无后端历史版本，跳过后端回滚")

        if frontend_changed:
            self.tag_frontend = prev_frontend_tag
            self.frontend_ref = f"{frontend_base}:{prev_frontend_tag}"
            log_info(f"回滚前端到: {self.frontend_ref}")
        elif prev_frontend_tag:
            log_ok(f"前端已是历史版本 tag={prev_frontend_tag}，无需重建")
        else:
            log_warn("无前端历史版本，跳过前端回滚")

        self.write_env_file()

        for service in (s for s in ("backend", "frontend")
                        if (s == "backend" and backend_changed) or (s == "frontend" and frontend_changed)):
            args = ["up", "-d", "--force-recreate"]
            args.append(service)
            log_info(f">>> 回滚重建 {service}")
            cp = self.run(self._compose() + args, check=False)
            if cp.returncode != 0:
                self._tail_logs(service)
                raise DeployError(f"{service} 回滚失败")

        if backend_changed and not self.assert_running_image("backend", self.backend_ref):
            raise DeployError("后端回滚自证失败：.env 已写 tag，但运行容器不是该镜像")
        if frontend_changed and not self.assert_running_image("frontend", self.frontend_ref):
            raise DeployError("前端回滚自证失败：.env 已写 tag，但运行容器不是该镜像")

        self.health_check()
        log_ok("回滚成功")
        log_info("如需恢复数据库，可执行：")
        log_info(f"  gunzip -c {self.backup_dir}/db_backup_*.sql.gz | docker exec -i $(docker compose -f {self.compose_file} ps -q postgres) "
                 f"psql -U {self.db_user} -d {self.db_name}")

    # ---------------------------------------------------------------- down
    def compose_down(self) -> None:
        self.run(self._compose() + ["down"])
        log_ok("容器已停止（命名卷保留）")

    # ---------------------------------------------------------------- 本地模式
    def local(self) -> None:
        self.preflight()
        if not self.opts.skip_build:
            if not shutil.which("npm"):
                raise DeployError("未安装 Node.js/npm：本地模式需要构建前端")
            frontend = REPO_ROOT / "frontend"
            log_info(">>> 构建前端...")
            if not (frontend / "node_modules").is_dir():
                self.run(["npm", "ci"], check=True, cwd=frontend)
            self.run(["npm", "run", "build"], check=True, cwd=frontend)
            log_ok("前端构建完成: dist/")

        self._validate_compose()
        # 本地模式不写 .env、不备份：直接用 backend/.env 作为 compose 的 env-file。
        # 先拉起数据库，再跑 go migrate（迁移要连 localhost:5432）。
        self.apply_data(self.plan_data())
        self.wait_postgres()
        if not self.opts.skip_migrate:
            log_info(">>> 执行数据库迁移（走后端镜像内的 migrate，不依赖宿主机 Go/5432 端口）...")
            self.run(self._compose() + [
                "run", "-T", "--rm", "--no-deps", "--entrypoint", "/app/bin/migrate",
                "backend", "up",
            ], check=True)
            log_ok("数据库迁移完成")
        self.apply_app(self.plan_app())
        self.health_check()


# ======================================================================
# CLI
# ======================================================================
def force_data_services_from_env(value: str) -> bool:
    """FORCE_DATA_SERVICES 的取值判据（CD 只给 testing 置 true）。

    与 `--force-data-services` 等价，但只能由**可抛弃环境**打开：冒烟栈每次重建，
    数据服务配置漂移是常态；判定仍走 decide_service，只在确有漂移时才 recreate。
    """
    return value.strip().lower() in ("1", "true", "yes")


def _parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(
        prog="deploy/deploy.py",
        description="叉车维修培训系统部署 CLI（按变更重启，只动变化的容器）",
    )
    p.add_argument("mode", nargs="?", default="deploy",
                   help="deploy（默认）/ rollback / local / down / plan / production（deploy 别名）")
    p.add_argument("--rollback", action="store_true", help="回滚到 backups/last-version.txt")
    p.add_argument("--plan", "--dry-run", dest="plan", action="store_true",
                   help="只打印变更计划，不重建运行中的容器")
    p.add_argument("--json", action="store_true", help="--plan 输出 JSON（stdout 只有 JSON）")
    p.add_argument("--force", action="store_true", help="重建全部应用服务（不含数据服务）")
    p.add_argument("--force-data-services", action="store_true", help="允许重建 postgres（危险）")
    p.add_argument("--remove-orphans", action="store_true", help="up 时移除孤儿容器")
    p.add_argument("--skip-build", action="store_true", help="本地模式跳前端构建")
    p.add_argument("--skip-migrate", action="store_true", help="跳过数据库迁移")
    p.add_argument("--migration-gate", action="store_true", help="dry-run：打印迁移失败后的去向（abort/continue）")
    p.add_argument("--rollback-plan", action="store_true", help="dry-run：打印回滚将写入 .env 的 tag")
    p.add_argument("--image-match", nargs=2, metavar=("ACTUAL", "EXPECTED"),
                   help="dry-run：运行镜像与期望镜像是否一致（退出码表达）")
    return p


def main(argv: Optional[List[str]] = None) -> int:
    global LOG_STREAM
    _configure_stdio()
    parser = _parser()
    args = parser.parse_args(argv)

    # 可抛弃环境（testing 冒烟）用环境变量放行数据服务重建：冒烟栈每次重建、数据可抛弃，
    # 数据服务配置漂移在那里是常态而不是风险（判据仍只在**确有漂移**时才 recreate，
    # 见 decide_service），而生产侧永远只认显式 --force-data-services。
    if force_data_services_from_env(os.environ.get("FORCE_DATA_SERVICES", "")):
        args.force_data_services = True

    # 纯判定入口：不读 .env、不碰 docker、不写文件。
    if args.migration_gate:
        print(migration_failure_action(os.environ.get("ALLOW_MIGRATION_FAILURE", "")))
        return 0
    if args.rollback_plan:
        print(f"IMAGE_TAG_BACKEND={rollback_tag(os.environ.get('PREVIOUS_BACKEND_IMAGE', ''))}")
        print(f"IMAGE_TAG_FRONTEND={rollback_tag(os.environ.get('PREVIOUS_FRONTEND_IMAGE', ''))}")
        return 0
    if args.image_match:
        actual, expected = args.image_match
        if images_equal(actual, expected):
            print("match")
            return 0
        print("mismatch")
        return 1

    mode = args.mode
    if mode == "production":
        mode = "deploy"
    if args.rollback:
        mode = "rollback"
    if mode == "plan":
        args.plan = True
        mode = "deploy"
    if mode not in ("deploy", "rollback", "local", "down"):
        parser.error(f"未知模式: {mode}")

    if args.json:
        LOG_STREAM = sys.stderr

    try:
        deployer = Deployer(mode, args)
        if args.plan:
            deployer.print_plan(json_output=args.json)
            return 0
        if mode == "down":
            deployer.compose_down()
            return 0
        if mode == "local":
            deployer.local()
            return 0
        if mode == "rollback":
            deployer.do_rollback()
            return 0
        deployer.deploy()
        return 0
    except DeployError as exc:
        log_error(str(exc))
        return 1
    except OSError as exc:
        log_error(f"文件/系统操作失败: {exc}")
        return 1
    except KeyboardInterrupt:
        log_error("已中断")
        return 130


if __name__ == "__main__":
    sys.exit(main())
