import { request } from "@playwright/test";
import fs from "node:fs";
import path from "node:path";

/**
 * 全量回归共用登录态：登录一次演示管理员账号，把会话令牌写成 Playwright
 * storageState。所有页面测试自动带上登录态；API 测试从 token.txt 读取后
 * 显式放到 Authorization 头里。
 */
async function globalSetup() {
  const baseURL = process.env.BASE_URL || "http://127.0.0.1:8080";
  const context = await request.newContext({ baseURL });
  const response = await context.post("/api/v1/auth/login", {
    data: {
      email: process.env.GEOLENS_DEMO_EMAIL || "demo@geolens.ai",
      password: process.env.GEOLENS_DEMO_PASSWORD || "demo1234",
    },
  });
  if (!response.ok()) {
    throw new Error(`E2E 登录失败: ${response.status()} ${await response.text()}`);
  }
  const { token } = (await response.json()) as { token: string };
  await context.dispose();

  const dir = path.join(process.cwd(), "e2e", ".auth");
  fs.mkdirSync(dir, { recursive: true });
  fs.writeFileSync(path.join(dir, "state.json"), JSON.stringify({
    cookies: [],
    origins: [{
      origin: baseURL,
      localStorage: [{ name: "geolens.auth.token", value: token }],
    }],
  }));
  fs.writeFileSync(path.join(dir, "token.txt"), token);
}

export default globalSetup;
