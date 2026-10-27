import { useEffect, useState, type ReactNode } from 'react';
import { Flex, Spin } from 'antd';
import { useNavigate } from 'react-router-dom';
import { getCookie } from '../api/request';
import { ApplicationProvider } from '../context/ApplicationContext';
import { PermissionProvider } from '../hooks/PermissionProvider';

interface AuthWrapperProps {
  children: ReactNode;
}

export default function AuthWrapper({ children }: AuthWrapperProps) {
  const navigate = useNavigate();
  const [checking, setChecking] = useState(true);

  useEffect(() => {
    const token = getCookie('access_token');
    if (!token || token.trim().length === 0) {
      const redirectPath = encodeURIComponent(window.location.pathname + window.location.search);
      navigate(`/login?redirect=${redirectPath}`);
    } else {
      setChecking(false);
    }
  }, [navigate]);

  if (checking) {
    return (
      <Flex justify="center" align="center" style={{ height: '100vh' }}>
        <Spin size="large" tip="加载中..." />
      </Flex>
    );
  }

  return (
    <ApplicationProvider>
      <PermissionProvider>{children}</PermissionProvider>
    </ApplicationProvider>
  );
}
