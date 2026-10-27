import { useEffect, useState } from 'react';
import { Modal, Select, Tag, Flex } from 'antd';
import { EmptyState } from './states';
import { accountApi, roleApi } from '../api/iam';
import { useApiErrorNotification } from '../hooks/useApiError';
import { useSubmitLock } from '../hooks/useSubmitLock';
import type { Account, Role } from '../types';
import { ACTIONS } from '../types/copywriting';

interface BindRolesModalProps {
  open: boolean;
  account: Account | null;
  onClose: () => void;
  onSuccess: () => void;
}

export function BindRolesModal({ open, account, onClose, onSuccess }: BindRolesModalProps) {
  const [roles, setRoles] = useState<Role[]>([]);
  const [selectedRoleCodes, setSelectedRoleCodes] = useState<string[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const showNotification = useApiErrorNotification();
  const { isSubmitting, run } = useSubmitLock();

  const initialRoleCodes = account?.roleCodeList ?? [];

  useEffect(() => {
    if (!open) return;
    let isMounted = true;
    setIsLoading(true);
    setSelectedRoleCodes(account?.roleCodeList ?? []);
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
      const selectedCodes = new Set(selectedRoleCodes);
      const initialCodes = new Set(initialRoleCodes);
      const addedRoleCodeList = [...selectedCodes].filter((code) => !initialCodes.has(code));
      const removedRoleCodeList = [...initialCodes].filter((code) => !selectedCodes.has(code));
      await accountApi.updateRoles(account.id, { addedRoleCodeList, removedRoleCodeList });
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
          <div style={{ color: 'rgba(0,0,0,0.45)', fontSize: 12 }}>选择角色（{selectedRoleCodes.length} 个已选）</div>
          <Select
            mode="multiple"
            style={{ width: '100%' }}
            loading={isLoading}
            value={selectedRoleCodes}
            onChange={setSelectedRoleCodes}
            options={roles.map((role) => ({ label: role.name, value: role.roleCode }))}
            tagRender={(props) => <Tag closable={props.closable} onClose={props.onClose}>{props.label}</Tag>}
            placeholder="请选择角色"
          />
        </Flex>
      )}
    </Modal>
  );
}