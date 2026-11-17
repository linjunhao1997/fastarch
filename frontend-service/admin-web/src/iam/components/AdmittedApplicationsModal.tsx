import { useEffect, useState } from 'react';
import { Modal, Select, Tag, Flex } from 'antd';
import { useQuery } from '@tanstack/react-query';
import { accountApi, applicationApi, profileApi } from '@/iam/api/iam';
import { useApiErrorNotification } from '@/shared/hooks/useApiError';
import { useSubmitLock } from '@/shared/hooks/useSubmitLock';
import type { Account, Application } from '@/iam/types';
import { ACTIONS } from '@/shared/types/copywriting';

interface AdmittedApplicationsModalProps {
  open: boolean;
  account: Account | null;
  onClose: () => void;
  onSuccess: () => void;
}

const hasHome = (id?: string): id is string => !!id && id !== '0';

/**
 * 设置用户的所属应用与额外准入应用（模型 A）：
 * - 所属应用默认进入、隐式可进入，不需要重复放在准入清单；
 * - application_ids 全量覆盖语义，只保存"除所属应用之外"的准入应用。
 */
export function AdmittedApplicationsModal({ open, account, onClose, onSuccess }: AdmittedApplicationsModalProps) {
  const [applications, setApplications] = useState<Application[]>([]);
  const [homeId, setHomeId] = useState('');
  const [admittedIds, setAdmittedIds] = useState<string[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const showNotification = useApiErrorNotification();
  const { isSubmitting, run } = useSubmitLock();
  // 数据边界（ADR 0006）：非 admin 不得移动用户的所属应用
  const { data: profile } = useQuery({ queryKey: ['profile', 'my'], queryFn: profileApi.get });
  const homeLocked = !(profile?.isAdmin ?? false);

  useEffect(() => {
    if (!open || !account) return;
    let isMounted = true;
    setIsLoading(true);
    Promise.all([applicationApi.list(1, 100), accountApi.get(account.id)])
      .then(([appsResponse, detail]) => {
        if (!isMounted) return;
        const home = hasHome(detail.homeApplicationId) ? detail.homeApplicationId : '';
        setApplications(appsResponse.applications);
        setHomeId(home);
        setAdmittedIds((detail.applications ?? []).map((app) => app.id).filter((id) => id !== home));
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

  const handleHomeChange = (value: string) => {
    setHomeId(value);
    // 所属应用隐式准入，不再重复占用额外准入名额
    setAdmittedIds((prev) => prev.filter((id) => id !== value));
  };

  const handleSubmit = () => {
    if (!account || !homeId) return;
    run(async () => {
      await accountApi.updateApplications(account.id, admittedIds, homeId);
      onSuccess();
      onClose();
    }).catch((error) => showNotification(error, '设置应用失败'));
  };

  const enabledApps = applications.filter((app) => app.status === 'enabled');
  const homeOptions = enabledApps.map((app) => ({
    value: app.id,
    label: `${app.name}（${app.code}）`,
  }));
  const admittedOptions = enabledApps
    .filter((app) => app.id !== homeId)
    .map((app) => ({ value: app.id, label: `${app.name}（${app.code}）` }));
  const nameOf = (id: string) => applications.find((app) => app.id === id)?.name ?? id;

  return (
    <Modal
      title={`设置应用：${account?.username ?? ''}`}
      open={open}
      onCancel={onClose}
      onOk={handleSubmit}
      confirmLoading={isSubmitting}
      okButtonProps={{ disabled: !homeId }}
      okText={ACTIONS.confirm}
      cancelText={ACTIONS.cancel}
    >
      <Flex vertical gap={12}>
        <div>
          <div style={{ marginBottom: 4 }}>所属应用（默认进入，隐式可进入）</div>
          <Select
            placeholder="选择所属应用"
            loading={isLoading}
            disabled={homeLocked}
            value={homeId || undefined}
            onChange={handleHomeChange}
            optionFilterProp="label"
            options={homeOptions}
            style={{ width: '100%' }}
          />
        </div>
        <div>
          <div style={{ marginBottom: 4 }}>额外准入应用（可多选，所属应用无需重复选择）</div>
          <Select
            mode="multiple"
            placeholder="选择额外可进入的应用，可留空"
            loading={isLoading}
            value={admittedIds}
            onChange={setAdmittedIds}
            optionFilterProp="label"
            options={admittedOptions}
            style={{ width: '100%' }}
          />
        </div>
        <Flex gap={4} wrap>
          {homeId && <Tag color="gold">所属：{nameOf(homeId)}</Tag>}
          {admittedIds.length > 0
            ? admittedIds.map((id) => <Tag key={id} color="geekblue">{nameOf(id)}</Tag>)
            : <Tag>仅可使用所属应用</Tag>}
        </Flex>
      </Flex>
    </Modal>
  );
}
