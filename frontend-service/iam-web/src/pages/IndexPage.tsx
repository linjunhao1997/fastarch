import { useEffect, useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Avatar, Button, Dropdown, Empty, Flex, Layout, Menu, Select, Spin, message } from 'antd';
import { MenuFoldOutlined, MenuUnfoldOutlined, UserOutlined } from '@ant-design/icons';
import { Navigate, Outlet, useLocation, useMatches, useNavigate } from 'react-router';
import AutoBreadcrumb from '../components/AutoBreadcrumb';
import {
  buildMenuItemsFromTree,
  collectRouteMeta,
  routes,
} from '../routers/router';
import { usePermissions } from '../hooks/permission-context';
import { menuApi } from '../api/iam';
import { useApplicationContext } from '../context/ApplicationContext';
import { layoutTokens, themeTokens } from '../theme';

const { Header, Sider, Content } = Layout;

function clearCookie(name: string): void {
  document.cookie = `${name}=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/;`;
}

function handleLogout(navigate: ReturnType<typeof useNavigate>) {
  clearCookie('access_token');
  localStorage.removeItem('access_token');
  localStorage.removeItem('current_application_id');
  navigate('/login');
}

// 路由直访守卫（specs/005-typed-permission-model T038）：未授权
// handle.permission 的路由重定向首页并提示无权限（quickstart §2 SC-003）。
function PermissionGate() {
  const { hasMenuPermission, loading } = usePermissions();
  const location = useLocation();
  const permission = useMatches()
    .map((match) => (match.handle as { permission?: string } | undefined)?.permission)
    .filter(Boolean)
    .pop();

  if (loading) {
    return (
      <Flex justify="center" align="center" style={{ height: '50vh' }}>
        <Spin size="large" tip="加载中..." />
      </Flex>
    );
  }
  if (permission && !hasMenuPermission(permission)) {
    return <UnauthorizedRedirect />;
  }
  return <Outlet key={location.pathname} />;
}

function UnauthorizedRedirect() {
  useEffect(() => {
    message.error('无权限访问该页面');
  }, []);
  return <Navigate to="/" replace />;
}

export default function IndexPage() {
  const [collapsed, setCollapsed] = useState(false);
  const navigate = useNavigate();
  const {
    applications,
    currentApplicationId,
    setCurrentApplicationId,
    loading: contextLoading,
    error: contextError,
    refetch: refetchContext,
  } = useApplicationContext();

  // 菜单树：按当前应用 + 登录态权限由服务端过滤（specs/008 T024）。
  // 切换应用时 placeholderData 保留原菜单，配合 Spin 呈现平滑过渡。
  const menuTreeQuery = useQuery({
    queryKey: ['my-menu-tree', currentApplicationId],
    queryFn: () => menuApi.listMyTree(currentApplicationId as string),
    enabled: !!currentApplicationId,
    placeholderData: (previous) => previous,
    retry: 1,
  });

  // 菜单由服务端菜单树直接渲染（specs/008 US3）：label/层级/顺序（display_order）
  // 均来自接口，路由表仅按 resource.key 补齐图标与跳转路径。
  const routeMeta = useMemo(() => collectRouteMeta(routes), []);
  const menuItems = useMemo(
    () => buildMenuItemsFromTree(menuTreeQuery.data?.roots ?? [], routeMeta),
    [menuTreeQuery.data, routeMeta],
  );

  const selectedKeys = location.pathname.split('/').filter(Boolean).slice(0, 2).join('/');

  const appOptions = useMemo(
    () =>
      applications.map((app) => ({
        value: app.id,
        label: app.status === 'enabled' ? app.name : `${app.name}（已停用）`,
        disabled: app.status !== 'enabled',
      })),
    [applications],
  );

  // 折叠态：图标按钮 + Dropdown 保持应用切换可用（specs/008 T023）
  const collapsedAppMenuItems = useMemo(
    () =>
      applications.map((app) => ({
        key: app.id,
        label: app.status === 'enabled' ? app.name : `${app.name}（已停用）`,
        disabled: app.status !== 'enabled',
      })),
    [applications],
  );

  const renderSiderBody = () => {
    // 个人信息加载中/失败、无归属应用：上下文级状态优先
    if (contextLoading) {
      return (
        <Flex justify="center" align="center" style={{ padding: 32 }}>
          <Spin tip="加载应用列表..." />
        </Flex>
      );
    }
    if (contextError) {
      return (
        <Flex vertical gap={8} align="center" style={{ padding: 24 }}>
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="获取个人信息失败" />
          <Button size="small" onClick={() => void refetchContext()}>重试</Button>
        </Flex>
      );
    }
    if (applications.length === 0 || !currentApplicationId) {
      return (
        <Empty
          image={Empty.PRESENTED_IMAGE_SIMPLE}
          description="暂无可用应用，请联系管理员分配"
          style={{ padding: 24 }}
        />
      );
    }

    // 菜单树异常（且无旧菜单可保留）时给出可重试的错误态
    if (menuTreeQuery.isError && !menuTreeQuery.data) {
      return (
        <Flex vertical gap={8} align="center" style={{ padding: 24 }}>
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="菜单加载失败" />
          <Button size="small" onClick={() => void menuTreeQuery.refetch()}>重试</Button>
        </Flex>
      );
    }
    if (menuItems.length === 0) {
      return (
        <Empty
          image={Empty.PRESENTED_IMAGE_SIMPLE}
          description="当前应用暂无可用菜单"
          style={{ padding: 24 }}
        />
      );
    }
    return (
      <Menu
        mode="inline"
        selectedKeys={[selectedKeys]}
        items={menuItems}
        onClick={({ key }) => {
          const meta = routeMeta.get(String(key));
          if (meta?.navigable) navigate(meta.path);
        }}
        style={{ height: `calc(100vh - ${layoutTokens.headerHeight}px)`, borderRight: 0 }}
      />
    );
  };

  return (
    <Layout style={{ height: '100vh', display: 'flex' }}>
      <Sider
        collapsible
        collapsed={collapsed}
        trigger={null}
        width={layoutTokens.sidebarWidth}
        style={{
          background: themeTokens.colorBgContainer,
          borderRight: `1px solid ${themeTokens.colorBorder}`,
          position: 'relative',
          boxShadow: themeTokens.sidebarShadow,
        }}
      >
        <Button
          type="text"
          icon={collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
          onClick={() => setCollapsed(!collapsed)}
          style={{
            position: 'absolute',
            right: collapsed ? -40 : -48,
            top: layoutTokens.sectionGap + 2,
            zIndex: 1,
            color: themeTokens.colorPrimary,
          }}
        />
        <div
          style={{
            height: layoutTokens.headerHeight,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            fontWeight: 600,
            color: themeTokens.colorPrimary,
            borderBottom: `1px solid ${themeTokens.colorBorder}`,
            padding: collapsed ? 0 : '0 8px',
          }}
        >
          {collapsed ? (
            <Dropdown
              menu={{
                items: collapsedAppMenuItems,
                selectable: true,
                selectedKeys: currentApplicationId ? [currentApplicationId] : [],
                onClick: ({ key }) => setCurrentApplicationId(key),
              }}
              disabled={applications.length === 0}
            >
              <Button type="text" title="切换应用">⚡</Button>
            </Dropdown>
          ) : (
            <Flex align="center" gap={8} style={{ width: '100%' }}>
              <Select
                variant="borderless"
                value={currentApplicationId ?? undefined}
                onChange={(value) => setCurrentApplicationId(value)}
                loading={menuTreeQuery.isFetching && !menuTreeQuery.isPlaceholderData}
                placeholder="选择应用"
                style={{ width: '100%' }}
                options={appOptions}
              />
            </Flex>
          )}
        </div>
        {renderSiderBody()}
        {menuTreeQuery.isFetching && !!menuTreeQuery.data ? (
          <Spin size="small" style={{ position: 'absolute', right: 12, bottom: 12 }} />
        ) : null}
      </Sider>
      <Layout>
        <Header
          style={{
            padding: `0 ${layoutTokens.contentPadding}px`,
            background: themeTokens.colorBgContainer,
            display: 'flex',
            justifyContent: 'flex-end',
            alignItems: 'center',
          }}
        >
          <Dropdown
            menu={{ items: [{ key: 'logout', label: '退出登录', onClick: () => handleLogout(navigate) }] }}
          >
            <Flex align="center" gap={12}>
              <Avatar style={{ backgroundColor: themeTokens.colorPrimary }} icon={<UserOutlined />} />
              <span>管理员</span>
            </Flex>
          </Dropdown>
        </Header>
        <div style={{ padding: themeTokens.sizeUnit * 2 }}>
          <AutoBreadcrumb />
        </div>
        <Content
          style={{
            padding: themeTokens.sizeUnit * 2,
            overflow: 'auto',
            background: themeTokens.colorBgLayout,
          }}
        >
          <PermissionGate />
        </Content>
      </Layout>
    </Layout>
  );
}
