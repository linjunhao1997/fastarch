/**
 * sender 登录跳转回归（ADR 0007 双控制台）：
 * 后端登录响应 Set-Cookie 为 HttpOnly。IAM 的 cookie 名是 authorization，与前端
 * document.cookie 写入的 access_token 不冲突；sender 硬编码 access_token，同名
 * HttpOnly cookie 使前端写入被浏览器静默忽略（RFC 6265 §5.3）且不可读。
 * 因此登录态必须持久化到 localStorage（request.ts 优先读它），否则后续请求
 * 无 Authorization 头，而网关又不转发 Cookie 头 → my-profile 401 → 登录流程
 * 抛错不跳转。本测试在 jsdom 中以"cookie 不可读"模拟该浏览器状态。
 */
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { App } from 'antd';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import SenderLoginPage from '@/sender/pages/SenderLoginPage';
import { resolvePostLoginPath } from '@/shared/pages/LoginPage';
import { profileApi } from '@/iam/api/iam';

const TOKEN = 'sender-test-token';

function okJson(body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });
}

describe('sender 登录态持久化与跳转', () => {
  const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
    const url = String(input);
    if (url.includes('/sender/authentication/password-auth')) {
      return okJson({ accessToken: TOKEN });
    }
    if (url.includes('/authentication/my-profile')) {
      return okJson({ id: '1', username: 'admin', isAdmin: false, homeApplicationId: '2' });
    }
    return okJson({});
  });

  beforeEach(() => {
    localStorage.clear();
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
          <MemoryRouter initialEntries={['/sender/login']}>
            <SenderLoginPage />
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

  it('登录成功后 token 持久化到 localStorage（HttpOnly cookie 不可读时的唯一可用存储）', async () => {
    await loginThroughPage();
    expect(localStorage.getItem('access_token')).toBe(TOKEN);
  });

  it('cookie 不可读时，后续请求仍携带 Authorization 头（模拟 HttpOnly 遮蔽）', async () => {
    await loginThroughPage();
    // 模拟浏览器真实状态：后端 Set-Cookie 为 HttpOnly，document.cookie 读不到，
    // 前端同名写入又被静默忽略 → 可读 cookie 为空。
    document.cookie = 'access_token=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/;';
    fetchMock.mockClear();
    await profileApi.get();
    const call = fetchMock.mock.calls.find(([url]) => String(url).includes('/authentication/my-profile'));
    expect(call).toBeTruthy();
    const headers = (call?.[1] as RequestInit).headers as Record<string, string>;
    expect(headers.Authorization).toBe(`Bearer ${TOKEN}`);
  });
});

describe('登录落点解析（redirect 污染与开放重定向防护）', () => {
  it('redirect 指向登录页时回退控制台首页', () => {
    // 401 handler 在登录页触发时会把当前页写进 redirect，重新登录后不应再回登录页
    expect(resolvePostLoginPath('/sender/login', '/sender')).toBe('/sender');
    expect(resolvePostLoginPath('/sender/login?redirect=%2Fsender', '/sender')).toBe('/sender');
    expect(resolvePostLoginPath('/login', '/')).toBe('/');
  });

  it('正常站内 redirect 保留，缺省回退首页', () => {
    expect(resolvePostLoginPath('/sender/records', '/sender')).toBe('/sender/records');
    expect(resolvePostLoginPath('/sender/records?page=2', '/sender')).toBe('/sender/records?page=2');
    expect(resolvePostLoginPath(null, '/sender')).toBe('/sender');
    expect(resolvePostLoginPath('', '/sender')).toBe('/sender');
  });

  it('站外与协议相对 redirect 一律拒绝', () => {
    expect(resolvePostLoginPath('https://evil.example', '/sender')).toBe('/sender');
    expect(resolvePostLoginPath('//evil.example', '/sender')).toBe('/sender');
  });
});
