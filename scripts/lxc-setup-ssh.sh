#!/bin/bash
# LXC 容器内配置 SSH（允许 root 密钥登录）+ 验证 Docker
set -e
export DEBIAN_FRONTEND=noninteractive

echo "=== 1. 验证 Docker ==="
docker version --format 'Server: {{.Server.Version}}'
docker info --format 'Driver: {{.Driver}} | Cgroup: {{.CgroupDriver}}'

echo ""
echo "=== 2. 安装 openssh-server ==="
apt-get install -y openssh-server

echo ""
echo "=== 3. 配置 SSH 加固 ==="
SSHD_CONFIG=/etc/ssh/sshd_config
sed -i 's/^#*PermitRootLogin.*/PermitRootLogin prohibit-password/' "$SSHD_CONFIG"
sed -i 's/^#*PasswordAuthentication.*/PasswordAuthentication no/' "$SSHD_CONFIG"
sed -i 's/^#*PubkeyAuthentication.*/PubkeyAuthentication yes/' "$SSHD_CONFIG"

mkdir -p /root/.ssh
chmod 700 /root/.ssh
touch /root/.ssh/authorized_keys
chmod 600 /root/.ssh/authorized_keys

echo ""
echo "=== 4. 重启 sshd ==="
systemctl restart sshd || systemctl restart ssh
systemctl enable sshd 2>/dev/null || systemctl enable ssh 2>/dev/null || true

echo ""
echo "=== 5. SSH 监听端口 ==="
ss -tlnp | grep :22 || netstat -tlnp | grep :22

echo ""
echo "✅ SSH 配置完成（仅允许密钥登录 root）"
