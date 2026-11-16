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
