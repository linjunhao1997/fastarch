import { App } from 'antd';
import { useCallback } from 'react';
import { buildConfirmDeleteMessage } from '../types/copywriting';

interface ConfirmDeleteOptions {
  entityType: string;
  entityName: string;
  onDelete: () => Promise<void>;
  onError?: (error: unknown) => void;
}

/**
 * 删除确认（通过 App.useApp() 的 modal 实例，跟随主题与上下文；
 * 静态 Modal.confirm 在部分挂载环境下不渲染，故统一走实例方法）。
 */
export function useConfirmDelete() {
  const { modal } = App.useApp();

  return useCallback(
    ({ entityType, entityName, onDelete, onError }: ConfirmDeleteOptions): void => {
      modal.confirm({
        title: `删除${entityType}`,
        content: buildConfirmDeleteMessage(entityType, entityName),
        okButtonProps: { danger: true },
        onOk: async () => {
          try {
            await onDelete();
          } catch (error) {
            onError?.(error);
            throw error;
          }
        },
      });
    },
    [modal],
  );
}
