.PHONY: build frontend backend dev clean test e2e

build: frontend backend
	cp geolens geolens-bin

frontend:
	cd frontend && npm install && npm run build

backend:
	GOPROXY=https://goproxy.cn,direct GOSUMDB=off go build -o geolens ./cmd/server/

dev:
	@echo "启动开发模式（前后端分离）..."
	@cd frontend && npx vite &
	@GEOLENS_FRONTEND_DIST=./frontend/dist go run ./cmd/server/

# WP-4.2：本地与 CI 共用的回归入口，任一失败即返回非零。
test:
	go test ./...
	cd frontend && npm run build

e2e:
	go test ./...
	cd frontend && npm ci && npm run build && npm run test:e2e

clean:
	rm -f geolens geolens-bin
	rm -rf frontend/dist frontend/node_modules
