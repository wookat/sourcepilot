import { expect, type ConsoleMessage, type Page } from '@playwright/test';

type GuardEntry = { type: string; text: string };

const allowedWarnings: RegExp[] = [
  /ResizeObserver loop completed with undelivered notifications/i,
  /Warning: \[antd: Modal\] Static function can not consume context like dynamic theme\. Please use 'App' component instead\./,
  // 已知历史问题，修复中：PR #91（antd destroyOnClose deprecation）、PR #94（/inventory 菜单重复 key）。合并后应删除以下两项。
  /Warning: \[antd: Modal\] `destroyOnClose` is deprecated\. Please use `destroyOnHidden` instead\./,
  /Warning: Duplicated key '\/inventory' used in Menu by path \[\/inventory\]/,
];

export class ConsoleGuard {
  private readonly errors: GuardEntry[] = [];
  private readonly warnings: GuardEntry[] = [];
  private readonly allowedForTest: RegExp[] = [];

  constructor(private readonly page: Page) {}

  /** 单测试级白名单：仅用于测试场景本身预期产生的控制台输出（如故意 mock 的 4xx 响应）。 */
  allowForTest(pattern: RegExp) {
    this.allowedForTest.push(pattern);
  }

  install() {
    this.page.on('pageerror', (error) => {
      this.errors.push({ type: 'pageerror', text: error.message });
    });
    this.page.on('console', (message) => this.recordConsole(message));
  }

  async expectNoFatalErrors() {
    const allowed = [...allowedWarnings, ...this.allowedForTest];
    const fatalErrors = this.errors.filter((entry) => !allowed.some((pattern) => pattern.test(entry.text)));
    const fatalWarnings = this.warnings.filter((entry) => !allowed.some((pattern) => pattern.test(entry.text)));
    expect([...fatalErrors, ...fatalWarnings], 'fatal console/page errors').toEqual([]);
  }

  entries() {
    return { errors: [...this.errors], warnings: [...this.warnings] };
  }

  private recordConsole(message: ConsoleMessage) {
    const text = message.text();
    if (message.type() === 'error') {
      this.errors.push({ type: 'console.error', text });
      return;
    }
    if (message.type() === 'warning' && /React|Warning|antd|Ant Design|useForm|unique "key"/i.test(text)) {
      this.warnings.push({ type: 'console.warning', text });
    }
  }
}
