import { test, expect } from "@playwright/test";
import { ConsolePage } from "../pages/console.page";

// WP-4.1：控制台 L0→L3 关键路径，逐页确认可达且口径声明可见。
const sections = [
  { label: "发布验收", title: "发布验收" },
  { label: "套餐账单", title: "套餐与账单" },
  { label: "监控发布", title: "监控与发布" },
  { label: "运行治理", title: "运行治理" },
  { label: "MCP 接入", title: "MCP 接入" },
];

for (const section of sections) {
  test(`控制台页面可达：${section.title}`, async ({ page }) => {
    const consolePage = new ConsolePage(page);
    await consolePage.openConsole();
    await consolePage.gotoSection(section.label, section.title);
    await expect(consolePage.heading(section.title)).toBeVisible();
  });
}

test("监控页展示五块看板与发布清单", async ({ page }) => {
  const consolePage = new ConsolePage(page);
  await consolePage.openConsole();
  await consolePage.gotoSection("监控发布", "监控与发布");
  // Δ-05 / EX-12：健康告警与业务告警分栏，suspect 批次不触发品牌表现变化。
  await expect(page.getByRole("heading", { name: /品牌表现告警/ })).toBeVisible();
  await expect(page.getByRole("heading", { name: /采集健康告警/ })).toBeVisible();
  // 该标题内嵌通过数 Badge，用 heading role 做稳定定位而非 exact 文本。
  await expect(page.getByRole("heading", { name: /发布检查清单/ })).toBeVisible();
});

// AEO 工作台整改：侧栏只保留核心链路；扩展模块移出主导航；系统管理默认收起。
test("侧栏为纯 AEO 工作台：核心链路可见、扩展模块不在主导航、系统管理默认收起", async ({ page }) => {
  const consolePage = new ConsolePage(page);
  await consolePage.openConsole();
  const sidebar = consolePage.sidebar;

  for (const label of ["概览", "品牌可见性", "竞品监测", "品牌事实库", "溯源数据", "账户套餐"]) {
    await expect(sidebar.getByRole("link", { name: label, exact: true })).toBeVisible();
  }
  for (const label of ["财务 AI", "付费广告", "Agent 管理"]) {
    await expect(sidebar.getByRole("link", { name: label, exact: true })).toHaveCount(0);
  }
  await expect(consolePage.getSystemNav()).not.toHaveAttribute("open", "");
});

test("AEO 核心链路可达：竞品监测 / 事实库 / 溯源数据", async ({ page }) => {
  const consolePage = new ConsolePage(page);
  await consolePage.openConsole();
  for (const { label, path } of [
    { label: "竞品监测", path: "/app/competitors" },
    { label: "品牌事实库", path: "/app/facts" },
    { label: "溯源数据", path: "/app/attribution" },
  ]) {
    await consolePage.nav(label).click();
    await expect(page).toHaveURL(new RegExp(path));
  }
});

test("扩展模块从概览卡片进入（路由保留，功能不删）", async ({ page }) => {
  await page.goto("/app");
  await expect(page.getByText("扩展模块")).toBeVisible();
  for (const label of ["财务 AI", "付费广告", "Agent 管理"]) {
    await expect(page.getByRole("link", { name: new RegExp(label) })).toBeVisible();
  }
});

test("概览不再展示 AEO 评分聚合分", async ({ page }) => {
  await page.goto("/app");
  await expect(page.getByText("AEO 评分")).toHaveCount(0);
  await expect(page.getByText("AEO 工作台")).toBeVisible();
});
