import { notification } from 'antd';
import type { ReactNode } from 'react';
import { NotificationContext } from './notification-context';

export function NotificationProvider({ children }: { children: ReactNode }) {
  const [api, contextHolder] = notification.useNotification();

  return (
    <NotificationContext.Provider value={api}>
      {contextHolder}
      {children}
    </NotificationContext.Provider>
  );
}
