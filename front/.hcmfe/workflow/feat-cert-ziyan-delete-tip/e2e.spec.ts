/**
 * feat-cert-ziyan-delete-tip（自研云证书-云上不支持删除，页面需提示）E2E 用例
 *
 * 对应手测清单：同目录 test.md（P0 主流程 / P1 异常分支）
 * 运行方式（本地 mock，不依赖真实环境）：
 *   npm run e2e -- .hcmfe/workflow/feat-cert-ziyan-delete-tip/e2e.spec.ts
 */
import { expect, test, type Page } from '@playwright/test';

import { mockApis, type IVerifyResource } from '../../../e2e/mocks/api';

// ---------- 页面入口 ----------
// 用业务路径，不要把域名写进来；baseURL 由 playwright.config.ts 提供
const BUSINESS_CERT_URL = '/#/business/cert?bizs=1'; // 业务视角：证书托管
const RESOURCE_CERT_URL = '/#/resource/resource?type=certs'; // 资源视角：资源管理 - 证书

// ---------- 选择器 ----------
// 证书名称在列表中唯一，用它定位整行；删除按钮是文本按钮（约 24x12px），必须直接 hover 按钮本身
// —— hover 操作列单元格会落在 .cell 的空白区域，不会进入 v-bk-tooltips 的宿主 span
const certRow = (page: Page, certName: string) =>
  page.locator('.bk-table-body tr').filter({ hasText: certName });
const deleteButton = (page: Page, certName: string) =>
  certRow(page, certName).getByRole('button', { name: '删除' });

/** 只保留业务访问权限，剥夺证书删除权限（避免整页被路由守卫拦到 403） */
const denyDeleteCert = (resource: IVerifyResource) =>
  !(resource.resource_type === 'cert' && resource.action === 'delete');

/** 统计点击后是否真的发出了证书删除请求 */
function watchDeleteRequests(page: Page): string[] {
  const urls: string[] = [];
  page.on('request', (req) => {
    if (req.method() === 'DELETE' && req.url().includes('certs')) urls.push(req.url());
  });
  return urls;
}

// ---------- 用例 ----------
test.describe('证书管理 - 自研云删除按钮（业务视角）', () => {
  test.beforeEach(async ({ page }) => {
    await mockApis(page);
    await page.goto(BUSINESS_CERT_URL);
  });

  test('P0-01 自研云证书的删除按钮置灰，点击不发起删除请求', async ({ page }) => {
    const deleteRequests = watchDeleteRequests(page);

    const button = deleteButton(page, 'ziyan-unassigned');
    await expect(button).toBeDisabled();
    // disabled 按钮不可 action，force 点击后也不应触发任何请求
    await button.click({ force: true, timeout: 3000 }).catch(() => {});

    expect(deleteRequests).toHaveLength(0);
  });

  test('P0-02 悬停置灰的删除按钮提示「自研云证书不允许删除，如有疑问，请联系C2000」', async ({ page }) => {
    await deleteButton(page, 'ziyan-unassigned').hover();

    await expect(page.getByText('自研云证书不允许删除，如有疑问，请联系C2000')).toBeVisible();
  });

  test('P0-04 非自研云（未分配业务）证书的删除按钮可点击，且不展示自研云提示', async ({ page }) => {
    const button = deleteButton(page, 'tcloud-unassigned');
    await expect(button).toBeEnabled();

    await button.hover();
    await expect(page.getByText('自研云证书不允许删除，如有疑问，请联系C2000')).toHaveCount(0);
  });

  test('P1-05 上传证书入口不受本次改动影响', async ({ page }) => {
    await expect(page.getByRole('button', { name: '上传证书' })).toBeEnabled();
  });
});

test.describe('证书管理 - 自研云删除按钮（资源视角）', () => {
  test.beforeEach(async ({ page }) => {
    await mockApis(page);
    await page.goto(RESOURCE_CERT_URL);
  });

  test('P0-03 资源视角下自研云证书的删除按钮同样置灰并提示「自研云证书不允许删除，如有疑问，请联系C2000」', async ({ page }) => {
    await expect(deleteButton(page, 'ziyan-unassigned')).toBeDisabled();

    await deleteButton(page, 'ziyan-unassigned').hover();
    await expect(page.getByText('自研云证书不允许删除，如有疑问，请联系C2000')).toBeVisible();
  });

  test('P1-01 自研云且已分配业务时，提示为「自研云证书不允许删除，如有疑问，请联系C2000」而非「已分配业务」', async ({ page }) => {
    await expect(deleteButton(page, 'ziyan-assigned')).toBeDisabled();

    await deleteButton(page, 'ziyan-assigned').hover();
    await expect(page.getByText('自研云证书不允许删除，如有疑问，请联系C2000')).toBeVisible();
    await expect(page.getByText('该证书已分配业务, 仅可在业务下操作')).toHaveCount(0);
  });

  test('P1-02 非自研云且已分配业务时，沿用既有的「已分配业务」提示', async ({ page }) => {
    await expect(deleteButton(page, 'tcloud-assigned')).toBeDisabled();

    await deleteButton(page, 'tcloud-assigned').hover();
    await expect(page.getByText('该证书已分配业务, 仅可在业务下操作')).toBeVisible();
  });
});

test.describe('证书管理 - 无删除权限', () => {
  test('P1-03 无删除权限时自研云证书的删除按钮仍为置灰态', async ({ page }) => {
    await mockApis(page, { authorized: denyDeleteCert });
    await page.goto(BUSINESS_CERT_URL);

    const deleteRequests = watchDeleteRequests(page);
    const button = deleteButton(page, 'ziyan-unassigned');
    await expect(button).toBeDisabled();
    await button.click({ force: true, timeout: 3000 }).catch(() => {});

    expect(deleteRequests).toHaveLength(0);
  });
});
