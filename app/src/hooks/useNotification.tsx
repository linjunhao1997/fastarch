import { notification } from "antd";
import React, { createContext, useContext, useCallback, ReactNode } from "react";

interface NotificationContextType {
  showError: (message: string, description?: string) => void;
}

const NotificationContext = createContext<NotificationContextType | undefined>(undefined);

export function NotificationProvider({ children }: { children: ReactNode }) {
  const [api, contextHolder] = notification.useNotification();

  const showError = useCallback((message: string, description?: string) => {
    api.error({
      message: message,
      description: description,
      duration: 5,
      placement: "topRight",
    });
  }, [api]);

  return (
    <NotificationContext.Provider value={{ showError }}>
      {contextHolder}
      {children}
    </NotificationContext.Provider>
  );
}

export function useNotification() {
  const context = useContext(NotificationContext);
  if (!context) {
    throw new Error("useNotification must be used within a NotificationProvider");
  }
  return context;
}