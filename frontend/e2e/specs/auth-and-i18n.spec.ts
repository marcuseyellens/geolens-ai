import { test, expect } from "@playwright/test";

// 页面级鉴权、套餐 CTA 落点与多语言切换的回归基线。
test.describe("匿名访问", () => {
  // 覆盖全局 storageState，模拟真正未登录的浏览器。
  test.use({ storageState: { cookies: [], origins: [] } });

  test("控制台与订阅页都重定向到登录页", async ({ page }) => {
    await page.goto("/app");
    await expect(page).toHaveURL(/\/login\?next=/);
    await expect(page.getByRole("heading", { name: "登录控制台" })).toBeVisible();

    await page.goto("/subscribe?plan=growth&kind=subscription");
    await expect(page).toHaveURL(/\/login\?next=/);
  });

  test("登录页可用演示账户完成登录", async ({ page }) => {
    await page.goto("/login?next=%2Fapp");
    await page.getByRole("button", { name: "填入演示账户" }).click();
    await page.locator('form button[type="submit"]').click();
    await expect(page).toHaveURL(/\/app$/);
    await expect(page.getByRole("link", { name: "概览", exact: true })).toBeVisible();
  });

  test("定价页套餐按钮指向订阅界面，不再是控制台", async ({ page }) => {
    await page.goto("/pricing");
    // L2 一次性审计、L3 复扫与订阅三档共 5 个 CTA，全部落到订阅界面。
    const subscribeLinks = page.locator('a[href^="/subscribe?"]');
    await expect(subscribeLinks).toHaveCount(5);
    for (const plan of ["one_time", "rescan", "starter", "growth", "scale"]) {
      await expect(page.locator(`a[href*="plan=${plan}"]`)).toHaveCount(1);
    }
    // 套餐表格里不能再有「进入控制台」；页头通用导航的 /app 链接不算套餐 CTA。
    await expect(page.locator('table a[href="/app"]')).toHaveCount(0);
  });

  test("语言切换把公开页与控制台文案切到英文并持久化", async ({ page }) => {
    await page.goto("/pricing");
    await expect(page.getByRole("heading", { name: "先用免费的，再决定要不要付费" })).toBeVisible();

    await page.getByTestId("locale-select").selectOption("en");
    await expect(page.getByRole("heading", { name: "Start free, then decide whether to pay" })).toBeVisible();
    expect(await page.locator("html").getAttribute("lang")).toBe("en");

    // 刷新后语言偏好保持
    await page.reload();
    await expect(page.getByRole("heading", { name: "Start free, then decide whether to pay" })).toBeVisible();

    await page.getByTestId("locale-select").selectOption("zh");
    await expect(page.getByRole("heading", { name: "先用免费的，再决定要不要付费" })).toBeVisible();
    expect(await page.locator("html").getAttribute("lang")).toBe("zh-CN");
  });
});

test("已登录控制台可用，退出登录后重新要求登录", async ({ page }) => {
  await page.goto("/app");
  await expect(page.getByRole("link", { name: "概览", exact: true })).toBeVisible();
  await expect(page.getByText("demo@geolens.ai")).toBeVisible();

  // 先换成本用例独立签发的会话再退出，避免撤销 globalSetup 共享给其他
  // worker 的令牌，导致并行测试集体 401。
  await page.evaluate(() => localStorage.removeItem("geolens.auth.token"));
  await page.goto("/login");
  await page.getByRole("button", { name: "填入演示账户" }).click();
  await page.locator('form button[type="submit"]').click();
  await expect(page).toHaveURL(/\/app$/);
  await page.getByRole("button", { name: "退出登录" }).click();
  await expect(page).toHaveURL(/\/login/);
});

test("订阅三步流程可完成开通", async ({ page }) => {
  await page.goto("/subscribe?plan=starter&kind=subscription");
  await expect(page.getByRole("heading", { name: "开通套餐，立刻开始检查" })).toBeVisible();
  await page.getByRole("button", { name: /下一步：确认内容/ }).click();
  await expect(page.getByRole("heading", { name: "第 2 步 · 确认内容" })).toBeVisible();
  await page.getByRole("button", { name: /下一步：付款/ }).click();
  await expect(page.getByRole("heading", { name: "第 3 步 · 付款" })).toBeVisible();
  await page.getByRole("button", { name: "立即开通", exact: true }).click();
  await expect(page.getByRole("heading", { name: /已开通/ })).toBeVisible();
});

test("控制台侧栏可切换英文", async ({ page }) => {
  await page.goto("/app/monitor");
  await expect(page.getByRole("heading", { name: "监控与发布" })).toBeVisible();
  await page.getByTestId("locale-select").selectOption("en");
  await expect(page.getByRole("heading", { name: "Monitoring & release" })).toBeVisible();
});

test("英文模式下主要页面不残留中文", async ({ page, context }) => {
  await context.addInitScript(() => {
    try { localStorage.setItem("geolens-locale", "en"); } catch { /* ignore */ }
  });
  const paths = ["/", "/pricing", "/app", "/app/monitor", "/app/acceptance", "/app/operations", "/app/mcp", "/app/billing", "/subscribe?plan=growth&kind=subscription", "/check"];
  for (const path of paths) {
    await page.goto(path);
    await page.waitForLoadState("networkidle");
    await expect(page.locator("html")).toHaveAttribute("lang", "en");
    const body = await page.locator("body").innerText();
    expect(body, `英文界面 ${path} 不应残留中文`).not.toMatch(/[\u4e00-\u9fff]/);
  }
});

test("语言选择器提供 8 种语言并真正切换版面文案", async ({ page }) => {
  await page.goto("/pricing");
  const select = page.getByTestId("locale-select").first();
  await expect(select.locator("option")).toHaveCount(8);

  await select.selectOption("ja");
  await expect(page.locator("html")).toHaveAttribute("lang", "ja");
  await expect(page.getByRole("heading", { name: "まず無料で試して、必要なら有料へ" })).toBeVisible();

  await select.selectOption("es");
  await expect(page.locator("html")).toHaveAttribute("lang", "es");
  await expect(page.getByRole("heading", { name: "Empieza gratis y decide después si pagas" })).toBeVisible();

  await select.selectOption("zh");
  await expect(page.locator("html")).toHaveAttribute("lang", "zh-CN");
  await expect(page.getByRole("heading", { name: "先用免费的，再决定要不要付费" })).toBeVisible();
});
