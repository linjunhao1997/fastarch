import { message, Alert } from "antd";
import React, { useCallback } from "react";
import { useNotification } from "./useNotification";

export function handleApiError(error: unknown, defaultMessage: string): void {
  console.error("API Error:", error);
  if (error instanceof Error && error.name === "UnauthorizedError") {
    return;
  }

  let errorMessage = defaultMessage;

  if (error instanceof Error) {
    errorMessage = error.message || defaultMessage;
  } else if (typeof error === "string") {
    errorMessage = error;
  } else if (error && typeof error === "object" && "message" in error) {
    errorMessage = String((error as { message?: unknown }).message) || defaultMessage;
  }

  message.error(errorMessage);
}

export function getApiErrorAlert(error: unknown, defaultMessage: string): React.ReactNode {
  console.error("API Error:", error);
  if (error instanceof Error && error.name === "UnauthorizedError") {
    return null;
  }

  let errorMessage = defaultMessage;

  if (error instanceof Error) {
    errorMessage = error.message || defaultMessage;
  } else if (typeof error === "string") {
    errorMessage = error;
  } else if (error && typeof error === "object" && "message" in error) {
    errorMessage = String((error as { message?: unknown }).message) || defaultMessage;
  }

  return (
    <Alert
      message="请求失败"
      description={errorMessage}
      type="error"
      showIcon
      style={{ marginBottom: 16 }}
    />
  );
}

export function useApiErrorNotification() {
  const { showError } = useNotification();

  const showNotification = useCallback((error: unknown, defaultMessage: string) => {
    console.error("[useApiErrorNotification] API Error:", error);
    if (error instanceof Error && error.name === "UnauthorizedError") {
      showError("请重新登录", "点击前往登录页", () => {
        window.location.replace("/login");
      });
      return;
    }

    let errorMessage = defaultMessage;

    if (error instanceof Error) {
      errorMessage = error.message || defaultMessage;
    } else if (typeof error === "string") {
      errorMessage = error;
    } else if (error && typeof error === "object" && "message" in error) {
      errorMessage = String((error as { message?: unknown }).message) || defaultMessage;
    }

    console.log("[useApiErrorNotification] Showing notification:", errorMessage);
    
    showError("请求失败", errorMessage);
  }, [showError]);

  return showNotification;
}

export function showApiErrorNotification(error: unknown, defaultMessage: string): void {
  console.error("[showApiErrorNotification] API Error:", error);
  if (error instanceof Error && error.name === "UnauthorizedError") {
    return;
  }

  let errorMessage = defaultMessage;

  if (error instanceof Error) {
    errorMessage = error.message || defaultMessage;
  } else if (typeof error === "string") {
    errorMessage = error;
  } else if (error && typeof error === "object" && "message" in error) {
    errorMessage = String((error as { message?: unknown }).message) || defaultMessage;
  }

  console.log("[showApiErrorNotification] Showing notification:", errorMessage);
  
  message.error(errorMessage);
}
