import type { Locator, Page } from "@playwright/test";

/** 落地页 page-object：只暴露稳定 locator，不在 spec 里写选择器。 */
export class LandingPage {
  readonly page: Page;
  readonly hero: Locator;
  readonly checkLink: Locator;
  readonly pricingLink: Locator;
  readonly neutralityNote: Locator;

  constructor(page: Page) {
    this.page = page;
    this.hero = page.getByRole("heading", { level: 1 }).first();
    this.checkLink = page.getByRole("link", { name: /免费检查|立即检查|开始检查/ }).first();
    this.pricingLink = page.getByRole("link", { name: /定价|价格/ }).first();
    this.neutralityNote = page.getByText(/不代表|不承诺|不保证/).first();
  }

  async open() {
    await this.page.goto("/");
    await this.hero.waitFor();
  }
}
