import { useEffect, useState } from 'react';
import { Modal, Select, Tag, Flex } from 'antd';
import { useApiErrorNotification } from '@/shared/hooks/useApiError';
import { useSubmitLock } from '@/shared/hooks/useSubmitLock';
import type { Account, Application } from '@/iam/types';
import { ACTIONS } from '@/shared/types/copywriting';
import { accountApi, applicationApi } from '@/iam/api/iam';

interface AdmittedApplicationsModalProps {
  open: boolean;
  account: Account | null;
  onClose: () => void;
  onSuccess: () => void;
}

/**
 * 设置用户的接入应用（ADR 0008）：
 * - 接入应用决定用户可切换的数据范围；applicationIds 全量覆盖语义。
 */
export function AdmittedApplicationsModal({ open, account, onClose, onSuccess }: AdmittedApplicationsModalProps) {
  const [applications, setApplications] = useState<Application[]>([]);
  const [admittedIds, setAdmittedIds] = useState<string[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const showNotification = useApiErrorNotification();
  const { isSubmitting, run } = useSubmitLock();

  useEffect(() => {
    if (!open || !account) return;
    let isMounted = true;
    setIsLoading(true);
    Promise.all([applicationApi.list(1, 100), accountApi.get(account.id)])
      .then(([appsResponse, detail]) => {
        if (!isMounted) return;
        setApplications(appsResponse.applications);
        setAdmittedIds((detail.applications ?? []).map((app) => app.id));
      })
      .catch((error) => {
        if (isMounted) showNotification(error, '获取应用/用户信息失败');
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
      await accountApi.updateApplications(account.id, admittedIds);
      onSuccess();
      onClose();
    }).catch((error) => showNotification(error, '设置应用失败'));
  };

  const enabledApps = applications.filter((app) => app.status === 'enabled');
  const admittedOptions = enabledApps.map((app) => ({ value: app.id, label: `${app.name}（${app.code}）` }));
  const nameOf = (id: string) => applications.find((app) => app.id === id)?.name ?? id;

  return (
    <Modal
      title={`设置应用：${account?.username ?? ''}`}
      open={open}
      onCancel={onClose}
      onOk={handleSubmit}
      confirmLoading={isSubmitting}
      okButtonProps={{ disabled: isLoading }}
      okText={ACTIONS.confirm}
      cancelText={ACTIONS.cancel}
    >
      <Flex vertical gap={12}>
        <div>
          <div style={{ marginBottom: 4 }}>接入应用（可多选，决定该用户可切换的数据范围）</div>
          <Select
            mode="multiple"
            placeholder="选择该用户可接入的应用"
            loading={isLoading}
            value={admittedIds}
            onChange={setAdmittedIds}
            optionFilterProp="label"
            options={admittedOptions}
            style={{ width: '100%' }}
          />
        </div>
        <Flex gap={4} wrap>
          {admittedIds.length > 0
            ? admittedIds.map((id) => <Tag key={id} color="geekblue">{nameOf(id)}</Tag>)
            : <Tag>未接入任何应用（该用户将无法选择数据范围）</Tag>}
        </Flex>
      </Flex>
    </Modal>
  );
}
