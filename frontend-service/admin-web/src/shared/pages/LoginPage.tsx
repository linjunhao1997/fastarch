import { useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { Alert, App, Button, Card, Form, Input } from 'antd';
import { useSearchParams } from 'react-router-dom';
import { authApi, profileApi } from '@/iam/api/iam';
import { setConsole, type ConsoleKinds } from '@/shared/api/app-context';
import { useSubmitLock } from '@/shared/hooks/useSubmitLock';
import { layoutTokens, themeTokens } from '@/shared/theme';
import { ACTIONS, buildSuccessMessage } from '@/shared/types/copywriting';

function clearCookie(name: string): void {
  document.cookie = `${name}=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/;`;
}

/**
 * 登录落点解析：redirect 仅接受站内路径（防开放重定向）；指向登录页时回退
 * 控制台首页——401 handler 在登录页触发（如登录流程内 profile 请求失败）会
 * 把当前页写进 redirect，重新登录后若照跳则永远回不到首页。
 */
export function resolvePostLoginPath(redirect: string | null, homePath: string): string {
  if (!redirect || !redirect.startsWith('/') || redirect.startsWith('//')) return homePath;
  const path = redirect.split(/[?#]/)[0];
  if (path === '/login' || path === '/sender/login') return homePath;
  return redirect;
}

/**
 * 登录页生成器：依控制台（ADR 0007）注入登录接口与登录后落点。
 * 登录成功后锁定 console（清离职 URL 残留的 current_application_id），
 * 拉取个人信息后跳转控制台首页。
 */
export function createLoginPage(options: {
  title: string;
  login: (data: { identifier: string; password: string }) => Promise<{ accessToken?: string }>;
  consoleKind: ConsoleKinds;
  homePath: string;
}) {
  return function LoginPage() {
    const [form] = Form.useForm();
    const [errorMsg, setErrorMsg] = useState<string | null>(null);
    const [searchParams] = useSearchParams();
    const { isSubmitting, run } = useSubmitLock();
    const queryClient = useQueryClient();
    const { message } = App.useApp();

    const onFinish = (values: { identifier: string; password: string }) => {
      setErrorMsg(null);
      run(async () => {
        const response = await options.login(values);
        if (!response) throw new Error('登录响应为空，请稍后重试');
        const token = response.accessToken;
        if (!token || token.trim().length === 0) throw new Error('登录响应格式错误：缺少有效的 token');
        // token 持久化以 localStorage 为准（request.ts/AuthWrapper 优先读它）：
        // sender 登录响应 Set-Cookie 为 HttpOnly 且同名 access_token，下面的
        // cookie 写入会被浏览器静默忽略（RFC 6265 §5.3），仅 localStorage 可靠。
        localStorage.setItem('access_token', token);
        document.cookie = `access_token=${token}; path=/`;
        // 登录即锁定控制台：菜单/字典前缀、重定向落点自此固定
        setConsole(options.consoleKind);
        try {
          const profile = await profileApi.get();
          queryClient.setQueryData(['profile', 'my'], profile);
        } catch {
          throw new Error('获取个人信息失败，请稍后重试或联系管理员');
        }
        // 先跳转再显示消息，避免延迟
        window.location.href = resolvePostLoginPath(searchParams.get('redirect'), options.homePath);
        message.success(buildSuccessMessage('登录'));
      }).catch((error) => {
        clearCookie('access_token');
        localStorage.removeItem('access_token');
        localStorage.removeItem('current_application_id');
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
          title={options.title}
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
  };
}

/** IAM 控制台登录页（logout 时也回到这里） */
export default createLoginPage({
  title: 'FastArch 身份认证平台',
  login: (data) => authApi.login(data),
  consoleKind: 'iam',
  homePath: '/',
});