import { useEffect, useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Avatar, Button, Dropdown, Empty, Flex, Layout, Menu, Select, Spin, Tooltip, message } from 'antd';
import { AppstoreOutlined, MenuFoldOutlined, MenuUnfoldOutlined, UserOutlined } from '@ant-design/icons';
import { Navigate, Outlet, useLocation, useMatches, useNavigate } from 'react-router';
import AutoBreadcrumb from '@/shared/components/AutoBreadcrumb';
import {
  buildMenuItemsFromTree,
  collectMenuLeafPaths,
  collectRouteMeta,
  routes,
} from '@/app/router';
import { usePermissions } from '@/shared/hooks/permission-context';
import { profileApi } from '@/iam/api/iam';
import { menuApi } from '@/shared/api/menu-dict-api';
import { clearRequestDataScope } from '@/shared/api/request';
import { clearConsole, getConsole } from '@/shared/api/app-context';
import { useConsole } from '@/shared/context/ConsoleContext';
import { useDataScope } from '@/shared/context/DataScopeContext';
import { layoutTokens, themeTokens } from '@/shared/theme';

const { Header, Sider, Content } = Layout;

function clearCookie(name: string): void {
  document.cookie = `${name}=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/;`;
}

function handleLogout(navigate: ReturnType<typeof useNavigate>) {
  clearCookie('access_token');
  localStorage.removeItem('access_token');
  clearConsole();
  clearRequestDataScope();
  navigate('/login');
}

// 路由直访守卫（重构 #4：权限码是唯一授权词汇）：未持有 handle.permission
// 声明的权限码时重定向首页并提示无权限；菜单可见性另由服务端 my-menu-tree 过滤。
function PermissionGate() {
  const { hasPermission, loading } = usePermissions();
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
  if (permission && !hasPermission(permission)) {
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
  const location = useLocation();
  const navigate = useNavigate();
  const { label: consoleLabel } = useConsole();
  const { permissionCodes } = usePermissions();
  const { appCode, current, applications, isAdmin, loading: scopeLoading, setAppCode } = useDataScope();
  // 右上角显示当前登录用户的昵称（缺失时回落用户名），与登录缓存同源。
  const { data: profile } = useQuery({ queryKey: ['profile', 'my'], queryFn: profileApi.get });
  const displayName = profile?.nickname || profile?.username || '用户';
  const enabledScopes = useMemo(
    () => applications.filter((app) => app.status === 'enabled'),
    [applications],
  );

  // 菜单树：服务端按宿主固定 appId 返回全量启用菜单（纯码门槛裁决）；
  // 可见性由前端按菜单 permission_code 与用户快照码过滤。
  const menuTreeQuery = useQuery({
    // queryKey 带 console 仅为按控制台隔离缓存（菜单/字典前缀由当前控制台决定）
    queryKey: ['my-menu-tree', getConsole()],
    queryFn: () => menuApi.listMyTree(),
    placeholderData: (previous) => previous,
    retry: 1,
  });

  // 菜单由服务端菜单树直接渲染（重构 #6）：层级/名称/图标/顺序取自 t_menu，
  // 跳转按 menu.path 匹配路由表；未知 path 跳过并告警。
  // 展示门槛（ADR 0002）：再按菜单 permission_code 与用户持有码二次过滤显隐。
  const routeMeta = useMemo(() => collectRouteMeta(routes), []);
  const menuItems = useMemo(
    () => buildMenuItemsFromTree(menuTreeQuery.data?.roots ?? [], routeMeta, permissionCodes),
    [menuTreeQuery.data, routeMeta, permissionCodes],
  );

  // 选中态：取当前 pathname 的最长已注册菜单 path 前缀（create/edit 等子页高亮列表项）
  const selectedKeys = useMemo(() => {
    const leafPaths = collectMenuLeafPaths(menuTreeQuery.data?.roots ?? []);
    const hit = leafPaths
      .filter((p) => location.pathname === p || location.pathname.startsWith(`${p}/`))
      .sort((a, b) => b.length - a.length)[0];
    return hit ? [hit] : [];
  }, [menuTreeQuery.data, location.pathname]);

  const renderSiderBody = () => {
    // 菜单树加载中（且无旧数据可保留）；isLoading 仅在真实请求中为 true，
    // 避免 enabled=false 时 isPending 恒真造成永久转圈
    if (menuTreeQuery.isLoading && !menuTreeQuery.data) {
      return (
        <Flex justify="center" align="center" style={{ padding: 32 }}>
          <Spin tip="加载菜单..." />
        </Flex>
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
          description="暂无可用菜单，请联系管理员分配角色菜单"
          style={{ padding: 24 }}
        />
      );
    }
    return (
      <Menu
        mode="inline"
        selectedKeys={selectedKeys}
        items={menuItems}
        onClick={({ key }) => {
          // 叶子菜单 key = menu.path（绝对路径），目录节点 key 以 dir- 前缀不可跳转
          if (typeof key === 'string' && key.startsWith('/')) navigate(key);
        }}
        style={{ height: `calc(100vh - ${layoutTokens.headerHeight + 72}px)`, borderRight: 0 }}
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
            whiteSpace: 'nowrap',
            overflow: 'hidden',
          }}
        >
          {collapsed ? 'FA' : consoleLabel}
        </div>
        {/* 数据范围切换：与控制台正交。选中项作为 X-Fa-App-Code 下发，决定列表/详情过滤哪个应用的数据。
            超管（ADR-0009）旁路数据范围，不展示切换器、也不发范围头——后端 resolveAppCode
            在 isAdmin 旁路前执行，admin 带陈旧 appCode 会被接入校验误拒。 */}
        {!isAdmin && (
          <div
            style={{
              padding: collapsed ? layoutTokens.sectionGap : `${layoutTokens.sectionGap}px ${layoutTokens.sectionGap * 2}px`,
              borderBottom: `1px solid ${themeTokens.colorBorder}`,
            }}
          >
            {collapsed ? (
              <Dropdown
                trigger={['click']}
                placement="bottomLeft"
                menu={{
                  selectedKeys: appCode ? [appCode] : [],
                  items: enabledScopes.map((app) => ({
                    key: app.code,
                    label: app.name,
                    onClick: () => setAppCode(app.code),
                  })),
                }}
              >
                <Tooltip title={`数据范围：${current?.name ?? '未选择'}`} placement="right">
                  <Button type="text" icon={<AppstoreOutlined />} block />
                </Tooltip>
              </Dropdown>
            ) : (
              <>
                <div style={{ fontSize: 12, color: 'rgba(0, 0, 0, 0.45)', marginBottom: 6 }}>
                  数据范围
                </div>
                <Select
                  value={appCode || undefined}
                  onChange={setAppCode}
                  loading={scopeLoading}
                  placeholder="请选择数据范围"
                  style={{ width: '100%' }}
                  options={enabledScopes.map((app) => ({ value: app.code, label: app.name }))}
                />
              </>
            )}
          </div>
        )}
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
              <span>{displayName}</span>
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
