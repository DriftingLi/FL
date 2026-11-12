"""EffectiveConfig module：统一配置解析（声明表 + 环境 secrets + 显式覆盖 → resolved map）。

统一优先级：显式覆盖 > 环境变量 > 声明表默认值。
DIAGNOSIS_* 等例外自然消解：空默认值 + 空环境 = 空结果，无需手工清单。

当前覆盖 deploy.py render_env_file 与 cd.yml 两层投影；
topology.go 声明表与 compose 插值兜底保持原状（后续可逐步收敛）。

兼容 ADR-0088 决策 8（Python 默认值只进内存不 source 不写 os.environ）。
"""
from __future__ import annotations

import re
import shlex
from dataclasses import dataclass, field
from typing import Dict, Optional

_ENV_LINE = re.compile(r"^export ([A-Za-z_][A-Za-z0-9_]*)=(.*)$")


def parse_env_text(text: str) -> Dict[str, str]:
    """解析 env.defaults 文本（shell source 格式）→ {名字: 值}。"""
    out: Dict[str, str] = {}
    for line in text.splitlines():
        stripped = line.strip()
        if not stripped or stripped.startswith("#"):
            continue
        m = _ENV_LINE.match(stripped)
        if not m:
            continue
        try:
            parts = shlex.split(m.group(2))
        except ValueError:
            continue
        out[m.group(1)] = parts[0] if parts else ""
    return out


def resolve_env(
    declarations: Dict[str, str],
    environment: Dict[str, str],
    overrides: Dict[str, str],
    derived: Optional[Dict[str, str]] = None,
) -> Dict[str, str]:
    """纯函数：三层输入 → resolved map。

    优先级：显式覆盖 > 环境变量 > 声明表默认值。
    derived 参数指定派生规则 {目标: 来源}，目标为空时取来源的 resolved 值。
    """
    result: Dict[str, str] = {}
    all_names = set(declarations) | set(environment) | set(overrides)
    if derived:
        all_names |= set(derived)

    for name in all_names:
        if name in overrides and overrides[name]:
            result[name] = overrides[name]
        elif name in environment and environment[name]:
            result[name] = environment[name]
        else:
            result[name] = declarations.get(name, "")

    # 派生规则：目标为空时取来源
    if derived:
        for target, source in derived.items():
            if not result.get(target) and result.get(source):
                result[target] = result[source]

    return result


@dataclass
class EffectiveConfig:
    """持有声明表与派生规则，供 deploy.py 调用。"""

    declarations: Dict[str, str] = field(default_factory=dict)
    derived: Dict[str, str] = field(default_factory=dict)

    def resolve(
        self,
        environment: Dict[str, str],
        overrides: Dict[str, str],
    ) -> Dict[str, str]:
        """计算 resolved map。"""
        return resolve_env(
            declarations=self.declarations,
            environment=environment,
            overrides=overrides,
            derived=self.derived,
        )

    @staticmethod
    def parse_declarations(text: str) -> Dict[str, str]:
        """解析 env.defaults 文本（shell source 格式）→ 声明表。"""
        out: Dict[str, str] = {}
        for line in text.splitlines():
            stripped = line.strip()
            if not stripped or stripped.startswith("#"):
                continue
            m = _ENV_LINE.match(stripped)
            if not m:
                continue
            try:
                parts = shlex.split(m.group(2))
            except ValueError:
                continue
            out[m.group(1)] = parts[0] if parts else ""
        return out

    @staticmethod
    def load_from_env_defaults(deploy_path) -> "EffectiveConfig":
        """从 deploy/env.defaults 加载声明表（文件不存在时返回空表）。"""
        from pathlib import Path
        path = Path(deploy_path) / "deploy" / "env.defaults"
        try:
            text = path.read_text(encoding="utf-8")
        except OSError:
            text = ""
        return EffectiveConfig(declarations=EffectiveConfig.parse_declarations(text))
