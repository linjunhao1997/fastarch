import { useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { Alert, Button, Card, Form, Input, message } from 'antd';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { authApi, profileApi } from '../api/iam';
import { useSubmitLock } from '../hooks/useSubmitLock';
import { layoutTokens } from '../theme';
import { ACTIONS, buildSuccessMessage } from '../types/copywriting';

function clearCookie(name: string): void {
  document.cookie = `${name}=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/;`;
}

export default function LoginPage() {
  const [form] = Form.useForm();
  const [errorMsg, setErrorMsg] = useState<string | null>(null);
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const { isSubmitting, run } = useSubmitLock();
  const queryClient = useQueryClient();

  const onFinish = (values: { email: string; password: string }) => {
    setErrorMsg(null);
    run(async () => {
      const response = await authApi.login(values);
      if (!response) throw new Error('登录响应为空，请稍后重试');
      const token = response.accessToken;
      if (!token || token.trim().length === 0) throw new Error('登录响应格式错误：缺少有效的 token');
      document.cookie = `access_token=${token}; path=/`;
      localStorage.setItem('access_token', token);
      // 先拉取个人信息初始化应用上下文（specs/008 T025）：失败给出可读提示，
      // 不静默进入无上下文状态；成功结果写入查询缓存供 ApplicationProvider 复用
      try {
        const profile = await profileApi.get();
        queryClient.setQueryData(['profile', 'my'], profile);
      } catch {
        throw new Error('获取个人信息失败，请稍后重试或联系管理员');
      }
      message.success(buildSuccessMessage('登录'));
      navigate(searchParams.get('redirect') || '/');
    }).catch((error) => {
      clearCookie('access_token');
      localStorage.removeItem('access_token');
      const errorMessage = error instanceof Error ? error.message : '登录失败，请检查邮箱和密码';
      setErrorMsg(errorMessage);
      message.error(errorMessage);
    });
  };

  const onFinishFailed = () => {
    message.error('请检查表单填写是否正确');
  };

  return (
    <div
      style={{
        display: 'flex',
        justifyContent: 'center',
        alignItems: 'center',
        height: '100vh',
        background: layoutTokens.backgroundGradient,
      }}
    >
      <Card
        title="FASTARCH"
        style={{ width: 400 }}
        styles={{ header: { textAlign: 'center', fontSize: 24 } }}
      >
        <Form form={form} layout="vertical" onFinish={onFinish} onFinishFailed={onFinishFailed}>
          {errorMsg ? <Alert message={errorMsg} type="error" showIcon style={{ marginBottom: 16 }} /> : null}
          <Form.Item
            label="邮箱"
            name="email"
            rules={[
              { required: true, message: '请输入邮箱' },
              { type: 'email', message: '请输入有效的邮箱格式' },
            ]}
          >
            <Input placeholder="请输入邮箱" />
          </Form.Item>
          <Form.Item label="密码" name="password" rules={[{ required: true, message: '请输入密码' }]}>
            <Input.Password placeholder="请输入密码" />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" loading={isSubmitting} block>
              {ACTIONS.confirm}
            </Button>
          </Form.Item>
        </Form>
      </Card>
    </div>
  );
}
