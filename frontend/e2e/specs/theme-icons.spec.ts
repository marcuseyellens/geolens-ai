import { test, expect } from "@playwright/test";

// 浅色主题图标可见性回归。曾出现真实缺陷：渐变令牌被 Tailwind 编译成
// background-color（linear-gradient 作为颜色值无效），于是白色图标落在
// 白色卡片上完全消失。这里把不变量固化下来。

function luminance(rgb: string): number {
  const [r, g, b] = (rgb.match(/\d+/g) || ["0", "0", "0"])
    .slice(0, 3)
    .map(Number)
    .map((v) => {
      const s = v / 255;
      return s <= 0.03928 ? s / 12.92 : Math.pow((s + 0.055) / 1.055, 2.4);
    });
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

function contrast(a: string, b: string): number {
  const l1 = luminance(a);
  const l2 = luminance(b);
  return (Math.max(l1, l2) + 0.05) / (Math.min(l1, l2) + 0.05);
}

test("浅色主题：白色图标有渐变底，语义色图标对比度达标", async ({ page }) => {
  await page.addInitScript(() => {
    try { localStorage.setItem("geolens-theme", "light"); } catch { /* ignore */ }
  });
  await page.goto("/app/monitor");
  await expect(page.getByRole("heading", { name: "监控与发布" })).toBeVisible();

  const sample = await page.evaluate(() => {
    const nearestBackground = (el: Element | null): string => {
      while (el) {
        const c = getComputedStyle(el as HTMLElement).backgroundColor;
        if (c && c !== "rgba(0, 0, 0, 0)") return c;
        el = el.parentElement;
      }
      return "rgb(255, 255, 255)";
    };
    const hasGradientAncestor = (el: Element | null): boolean => {
      while (el) {
        if (getComputedStyle(el as HTMLElement).backgroundImage.includes("linear-gradient")) return true;
        el = el.parentElement;
      }
      return false;
    };
    const icons = [...document.querySelectorAll("svg.lucide")];
    const white = icons.filter((s) => getComputedStyle(s).color === "rgb(255, 255, 255)");
    const semanticColors = ["rgb(5, 150, 105)", "rgb(180, 83, 9)", "rgb(220, 38, 38)"];
    const semantic = icons
      .filter((s) => semanticColors.includes(getComputedStyle(s).color))
      .map((s) => ({ color: getComputedStyle(s).color, bg: nearestBackground(s.parentElement) }));
    return {
      whiteCount: white.length,
      gradientCount: white.filter((s) => hasGradientAncestor(s.parentElement)).length,
      semantic,
      strokeWidth: getComputedStyle(icons[0]).strokeWidth,
    };
  });

  expect(sample.whiteCount).toBeGreaterThan(0);
  // 每个白色图标都必须有真实渐变底，否则就是白底白图标。
  expect(sample.gradientCount).toBe(sample.whiteCount);
  expect(sample.strokeWidth).toBe("2.25px");
  for (const row of sample.semantic) {
    expect(contrast(row.color, row.bg)).toBeGreaterThanOrEqual(3);
  }
});

test("深色主题仍使用原始描边权重", async ({ page }) => {
  await page.addInitScript(() => {
    try { localStorage.setItem("geolens-theme", "dark"); } catch { /* ignore */ }
  });
  await page.goto("/app/monitor");
  await expect(page.getByRole("heading", { name: "监控与发布" })).toBeVisible();
  const width = await page.evaluate(
    () => getComputedStyle(document.querySelector("svg.lucide")!).strokeWidth,
  );
  expect(width).toBe("2px");
});
