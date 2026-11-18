import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import AuthWrapper from '@/shared/components/AuthWrapper';
import { usePermissions } from '@/shared/hooks/permission-context';

vi.mock('@/iam/api/iam', () => ({
  getMyPermissions: vi.fn().mockResolvedValue({ permissionCodeList: [] }),
}));

function PermissionConsumer() {
  const { loading } = usePermissions();
  return <div>permission-consumer:{String(loading)}</div>;
}

describe('AuthWrapper 权限上下文装配', () => {
  beforeEach(() => {
    document.cookie = 'access_token=e2e-token; path=/';
  });

  it('受保护路由下的 usePermissions 不应抛出 Provider 缺失错误', async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });

    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter initialEntries={['/']}>
          <AuthWrapper consoleKind="iam">
            <PermissionConsumer />
          </AuthWrapper>
        </MemoryRouter>
      </QueryClientProvider>,
    );

    expect(await screen.findByText(/permission-consumer:/)).toBeTruthy();
  });
});
