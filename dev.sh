#!/bin/bash
set -e

cd "$(dirname "$0")"

echo "=== GeoLens AI 开发启动 ==="

# 检查前端依赖
if [ ! -d "frontend/node_modules" ]; then
  echo "安装前端依赖..."
  cd frontend && npm install && cd ..
fi

# 构建前端
echo "构建前端..."
cd frontend && npm run build && cd ..

# 编译后端
echo "编译后端..."
GOPROXY=https://goproxy.cn,direct GOSUMDB=off go build -o geolens ./cmd/server/

# 启动服务
echo "启动服务 http://localhost:8080"
echo "API: http://localhost:8080/api/v1"
echo ""
echo "按 Ctrl+C 停止"
echo ""

export GEOLENS_FRONTEND_DIST=./frontend/dist
./geolens
