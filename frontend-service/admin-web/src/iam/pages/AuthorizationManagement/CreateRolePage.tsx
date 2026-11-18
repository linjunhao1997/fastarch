import { useEffect } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Alert, Button, Card, Form, Input, Select, message } from 'antd';
import { useNavigate } from 'react-router';
import { applicationApi, profileApi, roleApi } from '@/iam/api/iam';
import { useApiErrorNotification } from '@/shared/hooks/useApiError';
import { useSubmitLock } from '@/shared/hooks/useSubmitLock';
import { ACTIONS } from '@/shared/types/copywriting';

export default function CreateRolePage() {
  const navigate = useNavigate();
  const [form] = Form.useForm();
  const queryClient = useQueryClient();
  const { isSubmitting, run } = useSubmitLock();
  const showNotification = useApiErrorNotification();

  // 角色归属应用 = 数据边界：非 admin 锁定为自身所属应用（服务端亦强制校验）
  const { data: profile } = useQuery({ queryKey: ['profile', 'my'], queryFn: profileApi.get });
  const { data: appsData, isLoading: appsLoading } = useQuery({
    queryKey: ['applications', 'create-role-options'],
    queryFn: () => applicationApi.list(1, 100),
  });
  const isAdmin = profile?.isAdmin ?? false;
  const lockedHomeId = profile && profile.homeApplicationId !== '0' ? profile.homeApplicationId : '';
  const homeLocked = !isAdmin;
  const cannotCreate = homeLocked && !lockedHomeId;

  useEffect(() => {
    if (homeLocked && lockedHomeId) {
      form.setFieldValue('applicationId', lockedHomeId);
    }
  }, [homeLocked, lockedHomeId, form]);

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
          <Alert type="warning" showIcon message="当前账号未设置所属应用，请联系管理员后再创建角色" />
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
            extra={homeLocked ? '非 admin 只能创建归属自己所属应用的角色' : undefined}
          >
            <Select
              options={appOptions}
              loading={appsLoading}
              disabled={homeLocked}
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
