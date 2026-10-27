import { createContext } from 'react';
import { useContext } from 'react';
import type { NotificationInstance } from 'antd/es/notification/interface';

export const NotificationContext = createContext<NotificationInstance | undefined>(undefined);

export function useNotification(): NotificationInstance {
  const notificationApi = useContext(NotificationContext);
  if (!notificationApi) throw new Error('useNotification must be used within NotificationProvider');
  return notificationApi;
}
