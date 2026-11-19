import { useEffect, useState, type ReactNode } from 'react';
import { Flex, Spin } from 'antd';
import { useLocation, useNavigate } from 'react-router-dom';
import { LOGIN_PATH } from '@/app/routes';
import { getStoredToken, clearRequestDataScope } from '@/shared/api/request';
import { setConsole } from '@/shared/api/app-context';
import { ConsoleProvider } from '@/shared/context/ConsoleContext';
import { DataScopeProvider } from '@/shared/context/DataScopeContext';
import { PermissionProvider } from '@/shared/hooks/PermissionProvider';

interface AuthWrapperProps {
  /** 受保护区所属控制台；由所在路由子树决定（统一登录后不再由登录页锁定） */
  consoleKind: 'iam' | 'sender';
  children: ReactNode;
}

export default function AuthWrapper({ consoleKind, children }: AuthWrapperProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const [checking, setChecking] = useState(true);

  // 控制台由路由子树确定，而非上一次会话的 localStorage：深链接直达 /sender/... 时
  // 也能拿到正确的菜单/字典前缀。此处同步写模块级值，确保子 Provider 首次渲染即读到。
  setConsole(consoleKind);

  useEffect(() => {
    const token = getStoredToken();
    if (!token || token.trim().length === 0) {
      // 统一登录：所有控制台共用 IAM 登录页。
      clearRequestDataScope();
      const redirectPath = encodeURIComponent(window.location.pathname + window.location.search);
      navigate(`${LOGIN_PATH}?redirect=${redirectPath}`);
    } else {
      setChecking(false);
    }
  }, [navigate, location.pathname, location.search]);

  if (checking) {
    return (
      <Flex justify="center" align="center" style={{ height: '100vh' }}>
        <Spin size="large" tip="加载中..." />
      </Flex>
    );
  }

  // ConsoleProvider 决定菜单/字典前缀；DataScopeProvider 决定业务数据归属；
  // PermissionProvider 决定能做什么操作。三者正交，均须挂在受保护区外层。
  return (
    <ConsoleProvider>
      <DataScopeProvider>
        <PermissionProvider>{children}</PermissionProvider>
      </DataScopeProvider>
    </ConsoleProvider>
  );
}
