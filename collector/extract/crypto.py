"""api_key 解密的 Python 镜像（#2673），与后端 internal/security/crypto.go 逐字节对齐。

格式：enc:v1:<base64(nonce[12] + ciphertext)>，key = sha256(masterKey)，AES-256-GCM。
无前缀 / 空密钥的历史明文原样返回（兼容加密改造前已入库的数据）。
cryptography 在调用时才 import：本机只跑纯判定单测时不必装该依赖。
"""

from __future__ import annotations

ENCRYPTED_PREFIX = "enc:v1:"


def decrypt_api_key(stored: str, master_key: str) -> str:
    if not master_key or not stored or not stored.startswith(ENCRYPTED_PREFIX):
        return stored

    import base64
    import hashlib

    from cryptography.exceptions import InvalidTag
    from cryptography.hazmat.primitives.ciphers.aead import AESGCM

    raw = base64.b64decode(stored[len(ENCRYPTED_PREFIX) :], validate=True)
    key = hashlib.sha256(master_key.encode("utf-8")).digest()
    nonce_size = 12  # GCM standard nonce size, same as Go gcm.NonceSize()
    if len(raw) < nonce_size:
        raise ValueError("密文长度不足")
    try:
        plain = AESGCM(key).decrypt(raw[:nonce_size], raw[nonce_size:], None)
    except InvalidTag as exc:
        raise ValueError("解密失败（请检查 SECRET_KEY 是否与加密时一致）") from exc
    return plain.decode("utf-8")
