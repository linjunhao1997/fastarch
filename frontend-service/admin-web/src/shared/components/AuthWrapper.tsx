import { useEffect, useState, type ReactNode } from 'react';
import { Flex, Spin } from 'antd';
import { useLocation, useNavigate } from 'react-router-dom';
import { getStoredToken } from '@/shared/api/request';
import { ConsoleProvider } from '@/shared/context/ConsoleContext';
import { PermissionProvider } from '@/shared/hooks/PermissionProvider';

interface AuthWrapperProps {
  /** 该受保护区所属控制台（登录即锁定，ADR 0007） */
  consoleKind: 'iam' | 'sender';
  children: ReactNode;
}

/** 各控制台的登录页路由 */
const LOGIN_PATHS: Record<'iam' | 'sender', string> = {
  iam: '/login',
  sender: '/sender/login',
};

export default function AuthWrapper({ consoleKind, children }: AuthWrapperProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const [checking, setChecking] = useState(true);

  useEffect(() => {
    const token = getStoredToken();
    if (!token || token.trim().length === 0) {
      const redirectPath = encodeURIComponent(window.location.pathname + window.location.search);
      navigate(`${LOGIN_PATHS[consoleKind]}?redirect=${redirectPath}`);
    } else {
      setChecking(false);
    }
  }, [navigate, consoleKind, location.pathname, location.search]);

  if (checking) {
    return (
      <Flex justify="center" align="center" style={{ height: '100vh' }}>
        <Spin size="large" tip="加载中..." />
      </Flex>
    );
  }

  // 两个控制台共用权限快照：IndexPage 的权限门与菜单过滤均依赖 usePermissions，
  // Provider 必须挂在受保护区外层（重构 #7 曾遗漏，导致运行时抛错）。
  return (
    <ConsoleProvider>
      <PermissionProvider>{children}</PermissionProvider>
    </ConsoleProvider>
  );
}