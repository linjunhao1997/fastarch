import { useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { Alert, App, Button, Card, Form, Input } from 'antd';
import { authApi, profileApi } from '@/iam/api/iam';
import { CONSOLE_SELECT_PATH } from '@/app/routes';
import { clearConsole } from '@/shared/api/app-context';
import { clearRequestDataScope } from '@/shared/api/request';
import { useSubmitLock } from '@/shared/hooks/useSubmitLock';
import { layoutTokens, themeTokens } from '@/shared/theme';
import { ACTIONS, buildSuccessMessage } from '@/shared/types/copywriting';

function clearCookie(name: string): void {
  document.cookie = `${name}=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/;`;
}

/**
 * 统一登录页：只由 IAM 校验凭据，登录成功后进入「控制台选择页」。
 *
 * 登录不再锁定 console（iam/sender 二选一）：console 由用户在选择页点击后锁定，
 * 数据范围（appCode）则由各控制台左上角切换。两者与权限码互相正交。
 * 登录时清空上一会话的 console 与 data scope，避免陈旧选择撞入新用户接入集合。
 */
export default function LoginPage() {
  const [form] = Form.useForm();
  const [errorMsg, setErrorMsg] = useState<string | null>(null);
  const { isSubmitting, run } = useSubmitLock();
  const queryClient = useQueryClient();
  const { message } = App.useApp();

  const onFinish = (values: { identifier: string; password: string }) => {
    setErrorMsg(null);
    run(async () => {
      const response = await authApi.login(values);
      if (!response) throw new Error('登录响应为空，请稍后重试');
      const token = response.accessToken;
      if (!token || token.trim().length === 0) throw new Error('登录响应格式错误：缺少有效的 token');

      // token 持久化以 localStorage 为准：cookie 仅兼容旧路径，HttpOnly 同名写入会被
      // 浏览器忽略且网关不转发 Cookie 头，request.ts / AuthWrapper 均优先读 localStorage。
      localStorage.setItem('access_token', token);
      document.cookie = `access_token=${token}; path=/`;
      clearConsole();
      clearRequestDataScope();

      try {
        const profile = await profileApi.get();
        queryClient.setQueryData(['profile', 'my'], profile);
      } catch {
        throw new Error('获取个人信息失败，请稍后重试或联系管理员');
      }

      window.location.href = CONSOLE_SELECT_PATH;
      message.success(buildSuccessMessage('登录'));
    }).catch((error) => {
      clearCookie('access_token');
      localStorage.removeItem('access_token');
      clearConsole();
      clearRequestDataScope();
      const errorMessage = error instanceof Error ? error.message : '登录失败，请检查账号和密码';
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
        background: `
          radial-gradient(circle at 20% 80%, rgba(37, 99, 235, 0.12) 0%, transparent 50%),
          radial-gradient(circle at 80% 20%, rgba(37, 99, 235, 0.08) 0%, transparent 50%),
          ${layoutTokens.backgroundGradient}
        `,
        position: 'relative',
        overflow: 'hidden',
      }}
    >
      <div style={{
        position: 'absolute',
        top: '10%',
        left: '5%',
        width: 300,
        height: 300,
        borderRadius: '50%',
        background: 'rgba(37, 99, 235, 0.06)',
        filter: 'blur(60px)',
      }} />
      <div style={{
        position: 'absolute',
        bottom: '15%',
        right: '10%',
        width: 200,
        height: 200,
        borderRadius: '50%',
        background: 'rgba(37, 99, 235, 0.08)',
        filter: 'blur(40px)',
      }} />
      <Card
        title="FastArch 统一登录"
        style={{ width: 400, boxShadow: '0 8px 32px rgba(37, 99, 235, 0.15)' }}
        styles={{ header: { textAlign: 'center', fontSize: 18, color: themeTokens.colorPrimary, fontWeight: 600 } }}
      >
        <Form form={form} layout="vertical" onFinish={onFinish} onFinishFailed={onFinishFailed}>
          {errorMsg ? <Alert message={errorMsg} type="error" showIcon style={{ marginBottom: 16 }} /> : null}
          <Form.Item
            label="账号"
            name="identifier"
            rules={[{ required: true, message: '请输入用户名或邮箱' }]}
          >
            <Input placeholder="用户名或邮箱" size="large" />
          </Form.Item>
          <Form.Item label="密码" name="password" rules={[{ required: true, message: '请输入密码' }]}>
            <Input.Password placeholder="请输入密码" size="large" />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" loading={isSubmitting} block size="large">
              {ACTIONS.confirm}
            </Button>
          </Form.Item>
        </Form>
      </Card>
    </div>
  );
}
