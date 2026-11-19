/**
 * 统一登录回归：
 *  1) 只有 IAM 登录端点，sender 不再有登录入口；
 *  2) token 必须落 localStorage（后端 Set-Cookie 为 HttpOnly，前端不可读）；
 *  3) 登录时清空上一会话 console / data scope，避免陈旧 appCode 撞新用户接入集合；
 *  4) profile 等自描述接口不带 X-Fa-App-Code，否则「先拿接入集合再校验范围」会死锁。
 */
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { App } from 'antd';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { CONSOLE_SELECT_PATH } from '@/app/routes';
import { profileApi } from '@/iam/api/iam';
import { setConsole } from '@/shared/api/app-context';
import LoginPage from '@/shared/pages/LoginPage';

const TOKEN = 'unified-test-token';

function okJson(body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });
}

describe('统一登录态持久化与范围清理', () => {
  const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
    const url = String(input);
    if (url.includes('/authentication/password-auth')) {
      return okJson({ accessToken: TOKEN });
    }
    if (url.includes('/authentication/my-profile')) {
      return okJson({
        id: '1',
        username: 'admin',
        isAdmin: false,
        applications: [{ id: '2', code: 'sender', name: '消息中心', status: 'enabled' }],
      });
    }
    return okJson({});
  });

  beforeEach(() => {
    localStorage.clear();
    localStorage.setItem('data_scope_app_code', 'stale-app');
    setConsole('sender');
    document.cookie = 'access_token=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/;';
    vi.stubGlobal('fetch', fetchMock);
    fetchMock.mockClear();
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
    localStorage.clear();
  });

  async function loginThroughPage() {
    render(
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
        <App>
          <MemoryRouter initialEntries={['/login']}>
            <LoginPage />
          </MemoryRouter>
        </App>
      </QueryClientProvider>,
    );
    fireEvent.change(screen.getByPlaceholderText('用户名或邮箱'), { target: { value: 'admin' } });
    fireEvent.change(screen.getByPlaceholderText('请输入密码'), { target: { value: 'password123' } });
    fireEvent.click(screen.getByRole('button', { name: /确\s*定/ }));
    await waitFor(() => {
      expect(fetchMock.mock.calls.some(([url]) => String(url).includes('/authentication/my-profile'))).toBe(true);
    });
  }

  it('登录后 token 落 localStorage，并清掉上一会话的 console 与 data scope', async () => {
    await loginThroughPage();
    expect(localStorage.getItem('access_token')).toBe(TOKEN);
    expect(localStorage.getItem('data_scope_app_code')).toBeNull();
    expect(localStorage.getItem('app_console')).toBeNull();
  });

  it('只调用 IAM 登录端点，不再走 sender 登录代理', async () => {
    await loginThroughPage();
    expect(fetchMock.mock.calls.some(([url]) => String(url).includes('/authentication/password-auth'))).toBe(true);
    expect(fetchMock.mock.calls.some(([url]) => String(url).includes('/sender/authentication'))).toBe(false);
  });

  it('profile 请求携带 Authorization，但显式跳过 X-Fa-App-Code', async () => {
    await loginThroughPage();
    // 模拟已选中数据范围：自描述接口仍不得携带 appCode。
    localStorage.setItem('data_scope_app_code', 'sender');
    fetchMock.mockClear();

    await profileApi.get();

    const call = fetchMock.mock.calls.find(([url]) => String(url).includes('/authentication/my-profile'));
    expect(call).toBeTruthy();
    const headers = (call?.[1] as RequestInit).headers as Record<string, string>;
    expect(headers.Authorization).toBe(`Bearer ${TOKEN}`);
    expect(headers['X-Fa-App-Code']).toBeUndefined();
  });

  it('登录成功后落点为控制台选择页', () => {
    expect(CONSOLE_SELECT_PATH).toBe('/console-select');
  });
});
