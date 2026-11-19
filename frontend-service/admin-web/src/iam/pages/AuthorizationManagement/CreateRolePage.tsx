import { useEffect } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Alert, Button, Card, Form, Input, Select, message } from 'antd';
import { useNavigate } from 'react-router';
import { applicationApi, profileApi, roleApi } from '@/iam/api/iam';
import { useApiErrorNotification } from '@/shared/hooks/useApiError';
import { useSubmitLock } from '@/shared/hooks/useSubmitLock';
import { useDataScope } from '@/shared/context/DataScopeContext';
import { ACTIONS } from '@/shared/types/copywriting';

export default function CreateRolePage() {
  const navigate = useNavigate();
  const [form] = Form.useForm();
  const queryClient = useQueryClient();
  const { isSubmitting, run } = useSubmitLock();
  const showNotification = useApiErrorNotification();
  const { current: currentScope } = useDataScope();

  // 角色归属应用 = 数据边界：非 admin 锁定为当前数据范围应用（服务端亦强制校验）
  const { data: profile } = useQuery({ queryKey: ['profile', 'my'], queryFn: profileApi.get });
  const { data: appsData, isLoading: appsLoading } = useQuery({
    queryKey: ['applications', 'create-role-options'],
    queryFn: () => applicationApi.list(1, 100),
  });
  const isAdmin = profile?.isAdmin ?? false;
  const lockedAppId = currentScope?.id ?? '';
  const appLocked = !isAdmin;
  const cannotCreate = appLocked && !lockedAppId;

  useEffect(() => {
    if (appLocked && lockedAppId) {
      form.setFieldValue('applicationId', lockedAppId);
    }
  }, [appLocked, lockedAppId, form]);

  const enabledApps = (appsData?.applications ?? []).filter((app) => app.status === 'enabled');
  const appOptions = enabledApps.map((app) => ({ value: app.id, label: `${app.name}（${app.code}）` }));

  const onFinish = async (values: { name: string; roleCode: string; applicationId: string }) => {
    run(async () => {
      await roleApi.create(values);
      message.success(`${ACTIONS.create}成功`);
      queryClient.invalidateQueries({ queryKey: ['roles'] });
      navigate('/authorization_management/role_management');
    }).catch((error) => showNotification(error, '角色创建失败'));
  };

  return (
    <div>
      <Card title="创建角色">
        {cannotCreate ? (
          <Alert type="warning" showIcon message="当前未选定数据范围应用，请联系管理员后再创建角色" />
        ) : null}
        <Form form={form} layout="vertical" onFinish={onFinish}>
          <Form.Item label="角色名称" name="name" rules={[{ required: true, message: '请输入角色名称' }]}>
            <Input />
          </Form.Item>
          <Form.Item
            label="角色编码"
            name="roleCode"
            rules={[{ required: true, message: '请输入角色编码' }]}
            extra="角色编码在所属应用内唯一，不同应用可使用同名编码"
          >
            <Input />
          </Form.Item>
          <Form.Item
            label="所属应用"
            name="applicationId"
            rules={[{ required: true, message: '请选择所属应用' }]}
            extra={appLocked ? '非 admin 只能创建归属当前数据范围应用的角色' : undefined}
          >
            <Select
              options={appOptions}
              loading={appsLoading}
              disabled={appLocked}
              placeholder="请选择所属应用"
            />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" loading={isSubmitting} disabled={cannotCreate}>
              {ACTIONS.create}
            </Button>
            <Button style={{ marginLeft: 8 }} onClick={() => navigate('/authorization_management/role_management')}>
              {ACTIONS.cancel}
            </Button>
          </Form.Item>
        </Form>
      </Card>
    </div>
  );
}
