import { test, expect } from "@playwright/test";
import { LandingPage } from "../pages/landing.page";

// WP-4.1：公开入口与中立性声明回归。
test("落地页展示主标题与中立性声明", async ({ page }) => {
  const landing = new LandingPage(page);
  await landing.open();
  await expect(landing.hero).toBeVisible();
  await expect(landing.neutralityNote).toBeVisible();
});

test("定价页与兑换页可从公开站点直接访问", async ({ page }) => {
  await page.goto("/pricing");
  await expect(page.getByRole("heading", { level: 1 }).first()).toBeVisible();
  await page.goto("/redeem");
  await expect(page.getByRole("heading", { name: "兑换码", exact: true })).toBeVisible();
  await expect(page.getByPlaceholder("AL-XXXX-0001")).toBeVisible();
});
