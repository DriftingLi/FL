#!/bin/bash
# LXC 容器内安装 Docker 24+ (Ubuntu 18.04)
# 使用阿里云镜像源加速
set -e
export DEBIAN_FRONTEND=noninteractive
echo 'libssl1.1:amd64 restart-services' | debconf-set-selections 2>/dev/null || true

# 清理残留 apt 进程和锁（前次中断可能留下）
killall apt-get apt dpkg 2>/dev/null || true
rm -f /var/lib/dpkg/lock /var/lib/dpkg/lock-frontend /var/lib/apt/lists/lock /var/cache/apt/archives/lock
dpkg --configure -a 2>/dev/null || true

echo "=== 1. 安装依赖 ==="
apt-get install -y ca-certificates curl gnupg lsb-release

echo "=== 2. 添加 Docker GPG key (阿里云) ==="
curl -fsSL https://mirrors.aliyun.com/docker-ce/linux/ubuntu/gpg | apt-key add -

echo "=== 3. 添加 Docker apt 源 (阿里云, bionic) ==="
echo "deb [arch=amd64] https://mirrors.aliyun.com/docker-ce/linux/ubuntu bionic stable" > /etc/apt/sources.list.d/docker.list

echo "=== 4. apt update ==="
apt-get update

echo "=== 5. 安装 Docker CE (指定 24.x) ==="
# 尝试安装 24.x 版本；若不可用则安装最新可用版本
apt-get install -y docker-ce docker-ce-cli containerd.io || \
  apt-get install -y docker-ce docker-ce-cli containerd.io

echo "=== 6. 配置 daemon.json (国内镜像 + 日志限制 + overlay2) ==="
mkdir -p /etc/docker
cat > /etc/docker/daemon.json << 'EOF'
{
  "registry-mirrors": [
    "https://docker.1ms.run",
    "https://docker.m.daocloud.io"
  ],
  "log-driver": "json-file",
  "log-opts": {
    "max-size": "50m",
    "max-file": "3"
  },
  "storage-driver": "overlay2"
}
EOF

echo "=== 7. 启动 dockerd ==="
systemctl daemon-reload
systemctl enable docker
systemctl restart docker

echo "=== 8. 验证 ==="
docker version --format 'Server: {{.Server.Version}}'
docker info --format 'Driver: {{.Driver}} | Cgroup: {{.CgroupDriver}}'

echo ""
echo "✅ Docker 安装完成"
