"""Python 解密与 Go security.EncryptSecret 的跨语言互操作单测（纯函数）。

Ciphertext 由真实 Go 代码 `security.EncryptSecret("sk-test-vector-2673", "unit-test-master-key")`
生成（2026-11-19），钉死 Python 镜像与后端加密格式一致：
    enc:v1:<base64(nonce[12] + ciphertext)>, key = sha256(masterKey), AES-256-GCM。
无前缀 / 空密钥的历史明文原样返回，与 Go DecryptSecret 的兼容分支一致。
"""

import unittest

from collector.extract.crypto import decrypt_api_key, ENCRYPTED_PREFIX


# Go security.EncryptSecret 真实产出的向量（非手算）。
GO_CIPHERTEXT = "enc:v1:ULpTLw09HcsKToqJbwY8777i33KFU0T7i1Rh1r3O82ub5SKMjvT9ODrltwB5YIs="
GO_MASTER_KEY = "unit-test-master-key"
GO_PLAINTEXT = "sk-test-vector-2673"


class DecryptAPITest(unittest.TestCase):
    def test_decrypts_go_encrypted_key(self):
        self.assertEqual(decrypt_api_key(GO_CIPHERTEXT, GO_MASTER_KEY), GO_PLAINTEXT)

    def test_wrong_key_fails_closed(self):
        with self.assertRaises(ValueError):
            decrypt_api_key(GO_CIPHERTEXT, "wrong-master-key")

    def test_plaintext_passthrough(self):
        # 历史明文（加密改造前入库）与空密钥：原样返回，与 Go 兼容分支一致。
        self.assertEqual(decrypt_api_key("sk-plain", GO_MASTER_KEY), "sk-plain")
        self.assertEqual(decrypt_api_key(GO_CIPHERTEXT, ""), GO_CIPHERTEXT)

    def test_prefix_constant_matches_backend(self):
        self.assertEqual(ENCRYPTED_PREFIX, "enc:v1:")


if __name__ == "__main__":
    unittest.main()
