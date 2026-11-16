import { useEffect, useState } from 'react';
import { Modal, Select, Tag, Flex } from 'antd';
import { accountApi, applicationApi } from '../api/iam';
import { useApiErrorNotification } from '../hooks/useApiError';
import { useSubmitLock } from '../hooks/useSubmitLock';
import type { Account, Application } from '../types';
import { ACTIONS } from '../types/copywriting';

interface AssignApplicationsModalProps {
  open: boolean;
  account: Account | null;
  onClose: () => void;
  onSuccess: () => void;
}

/**
 * 分配用户可进入的应用（多应用裁决 #8）：全量覆盖语义。
 * 已绑定清单从 GetAccount 详情读取（以详情为准，列表回填仅供展示）。
 */
export function AssignApplicationsModal({ open, account, onClose, onSuccess }: AssignApplicationsModalProps) {
  const [applications, setApplications] = useState<Application[]>([]);
  const [selectedIds, setSelectedIds] = useState<string[]>([]);
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
        setSelectedIds(detail.applications?.map((app) => app.id) ?? []);
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
      await accountApi.updateApplications(account.id, selectedIds);
      onSuccess();
      onClose();
    }).catch((error) => showNotification(error, '分配应用失败'));
  };

  return (
    <Modal
      title={`分配应用：${account?.username ?? ''}`}
      open={open}
      onCancel={onClose}
      onOk={handleSubmit}
      confirmLoading={isSubmitting}
      okText={ACTIONS.confirm}
      cancelText={ACTIONS.cancel}
    >
      <Flex vertical gap={12}>
        <Select
          mode="multiple"
          placeholder="选择用户可进入的应用（可多选，可留空）"
          loading={isLoading}
          value={selectedIds}
          onChange={setSelectedIds}
          optionFilterProp="label"
          options={applications.map((app) => ({
            value: app.id,
            label: app.status === 'enabled' ? `${app.name}（${app.code}）` : `${app.name}（${app.code}·已停用）`,
            disabled: app.status !== 'enabled',
          }))}
        />
        <Flex gap={4} wrap>
          {selectedIds.length > 0
            ? selectedIds.map((id) => {
              const app = applications.find((a) => a.id === id);
              return <Tag key={id} color="geekblue">{app ? `${app.name}（${app.code}）` : id}</Tag>;
            })
            : <Tag>未分配：该用户登录后无可进入应用</Tag>}
        </Flex>
      </Flex>
    </Modal>
  );
}
