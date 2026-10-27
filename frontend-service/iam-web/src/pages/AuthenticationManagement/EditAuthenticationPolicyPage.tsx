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

interface AuthPolicy {
  id: string;
  type: string;
  name: string;
  implementationId: string;
}

const policies: AuthPolicy[] = [
  { id: '1', type: 'sso', name: '企业 SSO 策略', implementationId: 'corp-sso' },
  { id: '2', type: 'mfa', name: '管理员 MFA 策略', implementationId: 'totp' },
  { id: '3', type: 'otp', name: '一次性密码策略', implementationId: 'email-otp' },
];

const policyTypeNames: Record<string, string> = {
  sso: 'SSO',
  mfa: 'MFA',
  otp: 'OTP',
};

export default function EditAuthenticationPolicyPage() {
  const navigate = useNavigate();
  const { id, type } = useParams<{ id: string; type: string }>();
  const { isSubmitting, run } = useSubmitLock();
  const policy = policies.find((item) => item.id === id && item.type === type);

  const onFinish = (values: AuthPolicyFormValues) => {
    void values;
    run(async () => {
      message.success(`${ACTIONS.save}成功`);
      navigate('/authentication_management/authentication_policy_list');
    }).catch(() => message.error('保存策略失败'));
  };

  const onFinishFailed = () => {
    message.error('请检查表单填写是否正确');
  };

  return (
    <Card title={`${ACTIONS.edit}${policyTypeNames[type ?? 'sso'] ?? '认证'}策略`}>
      <Form<AuthPolicyFormValues>
        layout="vertical"
        onFinish={onFinish}
        onFinishFailed={onFinishFailed}
        initialValues={policy ?? { type: type ?? 'sso', name: '', implementationId: '' }}
      >
        <Form.Item label="策略类型" name="type" rules={[{ required: true, message: '请选择策略类型' }]}>
          <Select options={Object.entries(policyTypeNames).map(([value, label]) => ({ value, label }))} />
        </Form.Item>
        <Form.Item label="策略名称" name="name" rules={[{ required: true, message: '请输入策略名称' }]}>
          <Input />
        </Form.Item>
        <Form.Item label="实现标识" name="implementationId" rules={[{ required: true, message: '请输入实现标识' }]}>
          <Input />
        </Form.Item>
        <Button type="primary" htmlType="submit" loading={isSubmitting}>{ACTIONS.save}</Button>
        <Button style={{ marginLeft: 8 }} onClick={() => navigate('/authentication_management/authentication_policy_list')}>
          {ACTIONS.cancel}
        </Button>
      </Form>
    </Card>
  );
}
