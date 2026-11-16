import type { Locator, Page } from "@playwright/test";

/** 控制台 page-object：侧栏导航与页面标题的稳定 locator。 */
export class ConsolePage {
  readonly page: Page;
  readonly sidebar: Locator;

  constructor(page: Page) {
    this.page = page;
    this.sidebar = page.locator("aside");
  }

  nav(label: string): Locator {
    return this.sidebar.getByRole("link", { name: label, exact: true });
  }

  heading(title: string): Locator {
    return this.page.getByRole("heading", { name: title, exact: true });
  }

  async openConsole() {
    await this.page.goto("/app");
    await this.sidebar.waitFor();
  }

  async gotoSection(label: string, title: string) {
    await this.nav(label).click();
    await this.heading(title).waitFor();
  }
}
