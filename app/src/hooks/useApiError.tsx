import { App } from 'antd';
import { useCallback } from 'react';
import { ApiError } from '../api/request';
import { FORBIDDEN_MESSAGE } from '../types/copywriting';

function resolveErrorMessage(error: unknown, defaultMessage: string): string {
  if (error instanceof Error) {
    if (error instanceof ApiError && (error.status === 403 || /permission denied/i.test(error.message))) {
      return FORBIDDEN_MESSAGE;
    }
    return error.message || defaultMessage;
  }
  if (typeof error === 'string') return error;
  if (error && typeof error === 'object' && 'message' in error) {
    const message = (error as { message?: unknown }).message;
    if (typeof message === 'string' && message) return message;
  }
  return defaultMessage;
}

export function useApiErrorNotification() {
  const { message } = App.useApp();

  return useCallback((error: unknown, defaultMessage: string) => {
    if (error instanceof Error && error.name === 'UnauthorizedError') return;
    message.error(resolveErrorMessage(error, defaultMessage));
  }, [message]);
}
