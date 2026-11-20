"""诚实 UA（#2677 判据 2）。

「诚实」= 能被站长识别并联系到人，而不是伪装成浏览器。所以 UA 里必须有产品标识
和联系方式；反过来，**不得**出现 Mozilla/Chrome/Safari 这类伪装 token。
"""

from __future__ import annotations

import os

PRODUCT = "ForkliftValuationCollector/1.0"
# 默认联系邮箱；生产可用 COLLECTOR_CONTACT_EMAIL 覆盖成运维真实邮箱。
DEFAULT_CONTACT = "ops@hrwai.cn"


def user_agent() -> str:
    """返回诚实 UA：产品名/版本 + 可联系邮箱。"""
    contact = os.environ.get("COLLECTOR_CONTACT_EMAIL") or DEFAULT_CONTACT
    return f"{PRODUCT} (+contact: {contact})"
