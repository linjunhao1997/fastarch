import { useEffect, useState } from 'react';
import { Modal, Select, Flex } from 'antd';
import { EmptyState } from '@/shared/components/states';
import { accountApi, roleApi } from '@/iam/api/iam';
import { useApiErrorNotification } from '@/shared/hooks/useApiError';
import { useSubmitLock } from '@/shared/hooks/useSubmitLock';
import type { Account, Role } from '@/iam/types';
import { ACTIONS } from '@/shared/types/copywriting';

interface BindRolesModalProps {
  open: boolean;
  account: Account | null;
  onClose: () => void;
  onSuccess: () => void;
}

export function BindRolesModal({ open, account, onClose, onSuccess }: BindRolesModalProps) {
  const [roles, setRoles] = useState<Role[]>([]);
  const [selectedRoleIds, setSelectedRoleIds] = useState<string[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const showNotification = useApiErrorNotification();
  const { isSubmitting, run } = useSubmitLock();

  // 初值与提交均按 roleId（ADR-0009）：账号详情的 roleIds 已由服务端按数据范围
  // 过滤，sender 侧看不到 iam 角色；跨应用同名码不再参与绑定，杜绝裸码越权删除。
  const initialRoleIds = account?.roleIds ?? [];

  useEffect(() => {
    if (!open) return;
    let isMounted = true;
    setIsLoading(true);
    setSelectedRoleIds(account?.roleIds ?? []);
    roleApi.list(1, 100)
      .then((response) => {
        if (isMounted) setRoles(response.roles);
      })
      .catch((error) => {
        if (isMounted) showNotification(error, '获取角色列表失败');
      })
      .finally(() => {
        if (isMounted) setIsLoading(false);
      });
    return () => {
      isMounted = false;
    };
  }, [account, open, showNotification]);

  const handleSubmit = () => {
    if (!account) return;
    run(async () => {
      const selected = new Set(selectedRoleIds);
      const initial = new Set(initialRoleIds);
      const addedRoleIds = [...selected].filter((id) => !initial.has(id));
      const removedRoleIds = [...initial].filter((id) => !selected.has(id));
      await accountApi.updateRoles(account.id, { addedRoleIds, removedRoleIds });
      onSuccess();
      onClose();
    });
  };

  return (
    <Modal
      title="绑定角色"
      open={open}
      onCancel={onClose}
      onOk={handleSubmit}
      confirmLoading={isSubmitting}
      okText={ACTIONS.save}
      cancelText={ACTIONS.cancel}
      destroyOnClose
      width={500}
    >
      {roles.length === 0 && !isLoading ? (
        <EmptyState description="暂无可绑定角色" action={{ text: '先去创建', onClick: onClose }} />
      ) : (
        <Flex vertical gap={8}>
          <div style={{ color: 'rgba(0,0,0,0.45)', fontSize: 12 }}>选择角色（{selectedRoleIds.length} 个已选）</div>
          <Select
            mode="multiple"
            style={{ width: '100%' }}
            loading={isLoading}
            value={selectedRoleIds}
            onChange={setSelectedRoleIds}
            options={roles.map((role) => ({ label: role.name, value: role.id }))}
            placeholder="请选择角色"
          />
        </Flex>
      )}
    </Modal>
  );
}
