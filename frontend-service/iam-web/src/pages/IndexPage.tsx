import { useEffect, useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Avatar, Button, Dropdown, Empty, Flex, Layout, Menu, Select, Spin, message } from 'antd';
import { MenuFoldOutlined, MenuUnfoldOutlined, UserOutlined } from '@ant-design/icons';
import { Navigate, Outlet, useLocation, useMatches, useNavigate } from 'react-router';
import AutoBreadcrumb from '../components/AutoBreadcrumb';
import {
  buildMenuItemsFromTree,
  collectMenuLeafPaths,
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
  const navigate = useNavigate();
  const { applications, currentApplicationId, setCurrentApplicationId, loading: contextLoading } = useApplicationContext();
  const { permissionCodes } = usePermissions();

  // 菜单树：服务端按应用上下文返回全量启用菜单（纯码门槛裁决）；
  // 可见性由前端按菜单 permission_code 与用户快照码过滤。
  const menuTreeQuery = useQuery({
    // 应用上下文在服务端（token→app 映射），queryKey 带 appId 仅为按应用隔离缓存
    queryKey: ['my-menu-tree', currentApplicationId],
    queryFn: () => menuApi.listMyTree(),
    enabled: !!currentApplicationId,
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

  // 切换应用：服务端绑定成功后强制回首页（右侧内容随新应用上下文重建，
  // 避免停留在上一应用的页面/数据上造成"没切过去"的错觉）
  const handleAppSwitch = async (id: string) => {
    try {
      await setCurrentApplicationId(id);
      navigate('/');
    } catch {
      // 错误提示已由 ApplicationContext 弹出；停留在原应用
    }
  };

  const renderSiderBody = () => {
    // 无可进入应用（非 admin 且未被分配准入）：引导联系管理员
    if (!contextLoading && applications.length === 0) {
      return (
        <Empty
          image={Empty.PRESENTED_IMAGE_SIMPLE}
          description="暂无可进入的应用，请联系管理员分配"
          style={{ padding: 24 }}
        />
      );
    }
    // 应用上下文尚未绑定完成（清单加载中 / SelectApplication 进行中）
    if (contextLoading || !currentApplicationId) {
      return (
        <Flex justify="center" align="center" style={{ padding: 32 }}>
          <Spin tip="绑定应用..." />
        </Flex>
      );
    }
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
                onClick: ({ key }) => void handleAppSwitch(key),
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
                onChange={(value) => void handleAppSwitch(value)}
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
