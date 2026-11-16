import { defineConfig, devices } from "@playwright/test";

// WP-4.1 端到端回归基线：page-object + 稳定 locator，覆盖 L0→L3 关键路径。
// CI 里由 workflow 先构建前端，再启动 Go 单服务（同时 serve 静态产物与 API）。
const baseURL = process.env.BASE_URL || "http://127.0.0.1:8080";

export default defineConfig({
  testDir: "./e2e/specs",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: process.env.CI ? 2 : undefined,
  reporter: process.env.CI ? [["github"], ["list"]] : "list",
  globalSetup: "./e2e/global-setup.ts",
  use: {
    baseURL,
    // 控制台与受保护 API 需要登录态；未登录场景在 auth.spec.ts 里用空 storageState 覆盖。
    storageState: "./e2e/.auth/state.json",
    trace: "on-first-retry",
    screenshot: "only-on-failure",
  },
  projects: [
    {
      name: "chromium",
      use: {
        ...devices["Desktop Chrome"],
        // 本机无法下载 Playwright chromium 时，可用 PW_CHANNEL=chrome 驱动系统 Chrome。
        ...(process.env.PW_CHANNEL ? { channel: process.env.PW_CHANNEL } : {}),
      },
    },
  ],
  webServer: {
    command: "cd .. && go run ./cmd/server/",
    url: baseURL + "/api/v1/health",
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
    env: {
      GEOLENS_FRONTEND_DIST: "frontend/dist",
      GEOLENS_DB: "/tmp/geolens-e2e/geolens.json",
      GEOLENS_DEMO: "1",
      GEOLENS_ADMIN_TOKEN: "e2e-admin-token",
      PORT: "8080",
    },
  },
});
