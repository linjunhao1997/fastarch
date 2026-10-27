import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { App as AntdApp, ConfigProvider } from 'antd';
import zhCN from 'antd/locale/zh_CN';
import { RouterProvider } from 'react-router-dom';
import { NotificationProvider } from './hooks/NotificationProvider';
import { router } from './routers/router';
import { themeTokens } from './theme';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      retry: 1,
    },
  },
});

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <ConfigProvider locale={zhCN} theme={{ token: themeTokens }}>
        <NotificationProvider>
          {/* antd App 上下文：App.useApp() 的 message/modal 实例依赖此挂载 */}
          <AntdApp>
            <RouterProvider router={router} />
          </AntdApp>
        </NotificationProvider>
      </ConfigProvider>
    </QueryClientProvider>
  );
}
