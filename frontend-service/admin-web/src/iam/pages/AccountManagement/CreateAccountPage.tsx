import { useEffect } from "react";
import { Form, Input, Button, Radio, Card, message, Select, Alert } from "antd";
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from "react-router-dom";
import { accountApi, applicationApi, profileApi } from "@/iam/api/iam";
import { useApiErrorNotification } from '@/shared/hooks/useApiError';
import { useSubmitLock } from '@/shared/hooks/useSubmitLock';
import { ACTIONS } from '@/shared/types/copywriting';

export default function CreateAccountPage() {
  const [form] = Form.useForm();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const showNotification = useApiErrorNotification();
  const { isSubmitting, run } = useSubmitLock();

  // 数据边界（ADR 0006）：非 admin 的所属应用锁定为操作者所属应用
  const { data: profile } = useQuery({ queryKey: ['profile', 'my'], queryFn: profileApi.get });
  const { data: appsData, isLoading: appsLoading } = useQuery({
    queryKey: ['applications', 'create-account-options'],
    queryFn: () => applicationApi.list(1, 100),
  });
  const isAdmin = profile?.isAdmin ?? false;
  const lockedHomeId = profile && profile.homeApplicationId !== '0' ? profile.homeApplicationId : '';
  const homeLocked = !isAdmin;
  const cannotCreate = homeLocked && !lockedHomeId;

  useEffect(() => {
    if (homeLocked && lockedHomeId) {
      form.setFieldValue('homeApplicationId', lockedHomeId);
    }
  }, [homeLocked, lockedHomeId, form]);

  const homeId = Form.useWatch('homeApplicationId', form) as string | undefined;
  const enabledApps = (appsData?.applications ?? []).filter((app) => app.status === 'enabled');
  const homeOptions = enabledApps.map((app) => ({ value: app.id, label: `${app.name}（${app.code}）` }));
  const admittedOptions = enabledApps
    .filter((app) => app.id !== homeId)
    .map((app) => ({ value: app.id, label: `${app.name}（${app.code}）` }));

  const handleHomeChange = (value: string) => {
    // 所属应用隐式准入，不再重复占用额外准入名额
    const admitted: string[] = form.getFieldValue('applicationIds') ?? [];
    form.setFieldValue('applicationIds', admitted.filter((id) => id !== value));
  };

  const onFinish = async (values: {
    username: string;
    nickname?: string;
    email?: string;
    password: string;
    status: string;
    homeApplicationId: string;
    applicationIds?: string[];
  }) => {
    run(async () => {
      // 初始密码必填（后端 bcrypt 摘要落库）；email 可空，留空即不设置
      await accountApi.create({
        username: values.username,
        nickname: values.nickname,
        email: values.email,
        password: values.password,
        status: values.status,
        homeApplicationId: values.homeApplicationId,
        applicationIds: values.applicationIds ?? [],
      });
      message.success(`${ACTIONS.create}成功`);
      queryClient.invalidateQueries({ queryKey: ['accounts'] });
      navigate('/account_management/account_list');
    }).catch((error) => showNotification(error, "用户创建失败"));
  };

  return (
    <div>
      <Card title="创建用户">
        {cannotCreate && (
          <Alert
            type="warning"
            showIcon
            style={{ marginBottom: 16 }}
            message="当前账号未设置所属应用，无法创建用户，请联系管理员。"
          />
        )}
        <Form
          form={form}
          layout="vertical"
          initialValues={{ status: "enabled" }}
          onFinish={onFinish}
        >
          <Form.Item
            label="用户名"
            name="username"
            rules={[{ required: true, message: "请输入用户名!" }]}
          >
            <Input placeholder="唯一登录名" />
          </Form.Item>

          <Form.Item label="昵称" name="nickname">
            <Input placeholder="显示名（可留空）" />
          </Form.Item>

          <Form.Item
            label="邮箱"
            name="email"
            rules={[{ type: "email", message: "请输入有效的邮箱格式!" }]}
          >
            <Input placeholder="可留空；设置后可用邮箱登录" />
          </Form.Item>

          <Form.Item
            label="初始密码"
            name="password"
            rules={[
              { required: true, message: "请输入初始密码" },
              { min: 8, message: "密码至少 8 位" },
            ]}
          >
            <Input.Password placeholder="至少 8 位" />
          </Form.Item>

          <Form.Item
            label="所属应用"
            name="homeApplicationId"
            rules={[{ required: true, message: "请选择所属应用" }]}
            extra={homeLocked ? "非 admin 只能创建本所属应用的用户" : "用户只能查看和操作所属应用的数据"}
          >
            <Select
              placeholder="选择所属应用（必填）"
              loading={appsLoading}
              disabled={homeLocked}
              options={homeOptions}
              optionFilterProp="label"
              onChange={handleHomeChange}
            />
          </Form.Item>

          <Form.Item
            label="准入应用"
            name="applicationIds"
            extra="额外可进入的应用，功能受权限码控制；所属应用隐式准入，无需选择"
          >
            <Select
              mode="multiple"
              placeholder="可留空"
              loading={appsLoading}
              options={admittedOptions}
              optionFilterProp="label"
            />
          </Form.Item>

          <Form.Item label="状态" name="status" rules={[{ required: true, message: "请选择状态" }]}>
            <Radio.Group>
              <Radio value="enabled">启用</Radio>
              <Radio value="disabled">禁用</Radio>
            </Radio.Group>
          </Form.Item>

          <Form.Item>
            <Button type="primary" htmlType="submit" loading={isSubmitting} disabled={cannotCreate}>
              {ACTIONS.create}
            </Button>
            <Button style={{ marginLeft: 8 }} onClick={() => navigate('/account_management/account_list')}>
              {ACTIONS.cancel}
            </Button>
          </Form.Item>
        </Form>
      </Card>
    </div>
  );
}
