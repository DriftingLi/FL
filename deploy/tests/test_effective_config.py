"""EffectiveConfig 纯函数测试：声明表 + 环境 secrets + 显式覆盖 → resolved map。

统一优先级规则：显式覆盖 > 环境变量 > 声明表默认值。
DIAGNOSIS_* 等例外不再靠手工清单，空默认值 + 空环境 = 空结果。
"""
from __future__ import annotations

import unittest
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))
from effective_config import EffectiveConfig, resolve_env


class TestResolveEnv(unittest.TestCase):
    """纯函数：输入 → resolved map。"""

    def test_empty_inputs_gives_empty_map(self):
        result = resolve_env(declarations={}, environment={}, overrides={})
        self.assertEqual(result, {})

    def test_declaration_defaults_are_used(self):
        result = resolve_env(
            declarations={"DOMAIN": "localhost", "LOG_LEVEL": "info"},
            environment={},
            overrides={},
        )
        self.assertEqual(result, {"DOMAIN": "localhost", "LOG_LEVEL": "info"})

    def test_environment_wins_over_default(self):
        result = resolve_env(
            declarations={"DOMAIN": "localhost"},
            environment={"DOMAIN": "hrwai.cn"},
            overrides={},
        )
        self.assertEqual(result["DOMAIN"], "hrwai.cn")

    def test_override_wins_over_everything(self):
        result = resolve_env(
            declarations={"DOMAIN": "localhost"},
            environment={"DOMAIN": "hrwai.cn"},
            overrides={"DOMAIN": "test.hrwai.cn"},
        )
        self.assertEqual(result["DOMAIN"], "test.hrwai.cn")

    def test_empty_environment_falls_back_to_default(self):
        result = resolve_env(
            declarations={"SMTP_HOST": "smtp.example.com"},
            environment={"SMTP_HOST": ""},
            overrides={},
        )
        self.assertEqual(result["SMTP_HOST"], "smtp.example.com")

    def test_empty_override_falls_back_to_environment(self):
        result = resolve_env(
            declarations={"DOMAIN": "localhost"},
            environment={"DOMAIN": "hrwai.cn"},
            overrides={"DOMAIN": ""},
        )
        self.assertEqual(result["DOMAIN"], "hrwai.cn")

    def test_secret_with_empty_default_resolves_to_empty(self):
        """DIAGNOSIS_* 类 secrets：空默认 + 空环境 = 空结果（不是死值）。"""
        result = resolve_env(
            declarations={"DIAGNOSIS_ASSISTANT_URL": "", "DIAGNOSIS_V2_API_KEY": ""},
            environment={},
            overrides={},
        )
        self.assertEqual(result["DIAGNOSIS_ASSISTANT_URL"], "")
        self.assertEqual(result["DIAGNOSIS_V2_API_KEY"], "")

    def test_secret_with_environment_resolves_to_value(self):
        result = resolve_env(
            declarations={"DIAGNOSIS_V2_API_KEY": ""},
            environment={"DIAGNOSIS_V2_API_KEY": "sk-abc123"},
            overrides={},
        )
        self.assertEqual(result["DIAGNOSIS_V2_API_KEY"], "sk-abc123")

    def test_unknown_env_var_is_included(self):
        """环境中存在但声明表没有的变量也要进 resolved map（cd.yml 传值）。"""
        result = resolve_env(
            declarations={},
            environment={"GITHUB_TOKEN": "ghp_xxx"},
            overrides={},
        )
        self.assertEqual(result["GITHUB_TOKEN"], "ghp_xxx")

    def test_full_priority_chain(self):
        """三层都有值时取 override > env > default。"""
        result = resolve_env(
            declarations={"A": "from-default", "B": "from-default", "C": "from-default"},
            environment={"B": "from-env", "C": "from-env"},
            overrides={"C": "from-override"},
        )
        self.assertEqual(result["A"], "from-default")
        self.assertEqual(result["B"], "from-env")
        self.assertEqual(result["C"], "from-override")

    def test_derived_fallback(self):
        """派生规则：SMTP_USERNAME 缺省时取 SMTP_FROM。"""
        result = resolve_env(
            declarations={"SMTP_USERNAME": "", "SMTP_FROM": "noreply@hrwai.cn"},
            environment={},
            overrides={},
            derived={"SMTP_USERNAME": "SMTP_FROM"},
        )
        self.assertEqual(result["SMTP_USERNAME"], "noreply@hrwai.cn")

    def test_derived_not_used_when_env_present(self):
        result = resolve_env(
            declarations={"SMTP_USERNAME": "", "SMTP_FROM": "noreply@hrwai.cn"},
            environment={"SMTP_USERNAME": "real-user@hrwai.cn"},
            overrides={},
            derived={"SMTP_USERNAME": "SMTP_FROM"},
        )
        self.assertEqual(result["SMTP_USERNAME"], "real-user@hrwai.cn")

    def test_derived_source_also_empty_gives_empty(self):
        result = resolve_env(
            declarations={"SMTP_USERNAME": "", "SMTP_FROM": ""},
            environment={},
            overrides={},
            derived={"SMTP_USERNAME": "SMTP_FROM"},
        )
        self.assertEqual(result["SMTP_USERNAME"], "")


class TestEffectiveConfig(unittest.TestCase):
    """EffectiveConfig 类：持有声明表，供 deploy.py 调用。"""

    def test_resolve_from_class(self):
        ec = EffectiveConfig(
            declarations={"DOMAIN": "localhost", "LOG_LEVEL": "info"},
            derived={"SMTP_USERNAME": "SMTP_FROM"},
        )
        result = ec.resolve(
            environment={"DOMAIN": "hrwai.cn"},
            overrides={"LOG_LEVEL": "debug"},
        )
        self.assertEqual(result["DOMAIN"], "hrwai.cn")
        self.assertEqual(result["LOG_LEVEL"], "debug")

    def test_declarations_from_env_defaults_text(self):
        """从 env.defaults 文本解析声明表。"""
        text = "\n".join([
            "export DOMAIN='localhost'",
            "export LOG_LEVEL='info'",
            "export LOG_FORMAT='console'",
        ])
        declarations = EffectiveConfig.parse_declarations(text)
        self.assertEqual(declarations, {
            "DOMAIN": "localhost",
            "LOG_LEVEL": "info",
            "LOG_FORMAT": "console",
        })

    def test_parse_declarations_ignores_comments_and_blanks(self):
        text = "\n".join([
            "# comment",
            "",
            "export A='1'",
            "  # indented comment",
            "export B='2'",
        ])
        declarations = EffectiveConfig.parse_declarations(text)
        self.assertEqual(declarations, {"A": "1", "B": "2"})


if __name__ == "__main__":
    unittest.main()
