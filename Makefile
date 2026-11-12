.PHONY: build frontend backend dev clean

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

clean:
	rm -f geolens geolens-bin
	rm -rf frontend/dist frontend/node_modules
