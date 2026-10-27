import { Button, Card, Form, Input, Select, message } from 'antd';
import { useNavigate, useParams } from 'react-router';
import { ACTIONS } from '../../types/copywriting';
import { useSubmitLock } from '../../hooks/useSubmitLock';

// 演示数据，认证策略后端契约待后续特性。
interface AuthPolicyFormValues {
  type: string;
  name: string;
  implementationId: string;
}

const policyTypeNames: Record<string, string> = {
  sso: 'SSO',
  mfa: 'MFA',
  otp: 'OTP',
};

export default function CreateAuthenticationPolicyPage() {
  const navigate = useNavigate();
  const { type } = useParams<{ type: string }>();
  const { isSubmitting, run } = useSubmitLock();

  const onFinish = (values: AuthPolicyFormValues) => {
    void values;
    run(async () => {
      message.success(`${ACTIONS.create}成功`);
      navigate('/authentication_management/authentication_policy_list');
    }).catch(() => message.error('创建策略失败'));
  };

  const onFinishFailed = () => {
    message.error('请检查表单填写是否正确');
  };

  return (
    <Card title={`${ACTIONS.create}${policyTypeNames[type ?? 'sso'] ?? '认证'}策略`}>
      <Form<AuthPolicyFormValues> layout="vertical" onFinish={onFinish} onFinishFailed={onFinishFailed} initialValues={{ type: type ?? 'sso' }}>
        <Form.Item label="策略类型" name="type" rules={[{ required: true, message: '请选择策略类型' }]}>
          <Select options={Object.entries(policyTypeNames).map(([value, label]) => ({ value, label }))} />
        </Form.Item>
        <Form.Item label="策略名称" name="name" rules={[{ required: true, message: '请输入策略名称' }]}>
          <Input />
        </Form.Item>
        <Form.Item label="实现标识" name="implementationId" rules={[{ required: true, message: '请输入实现标识' }]}>
          <Input />
        </Form.Item>
        <Button type="primary" htmlType="submit" loading={isSubmitting}>{ACTIONS.create}</Button>
        <Button style={{ marginLeft: 8 }} onClick={() => navigate('/authentication_management/authentication_policy_list')}>
          {ACTIONS.cancel}
        </Button>
      </Form>
    </Card>
  );
}
