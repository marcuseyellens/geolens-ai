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

  getSystemNav(): Locator {
    return this.page.getByTestId("system-nav");
  }

  /** 展开「系统管理」折叠组（运维/治理页面已收敛于此）。 */
  async expandSystemNav() {
    const details = this.page.getByTestId("system-nav");
    if ((await details.getAttribute("open")) === null) {
      await details.locator("summary").click();
    }
  }

  async gotoSection(label: string, title: string) {
    const link = this.nav(label);
    if (!(await link.isVisible())) await this.expandSystemNav();
    await link.click();
    await this.heading(title).waitFor();
  }
}
