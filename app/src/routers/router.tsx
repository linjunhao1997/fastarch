import { Suspense, lazy, type ReactNode } from 'react';
import { Link, createBrowserRouter } from 'react-router-dom';
import type { RouteObject } from 'react-router-dom';
import { Spin } from 'antd';
import type { MenuProps } from 'antd';
import { AppstoreOutlined, KeyOutlined, SafetyOutlined, SecurityScanOutlined, UserSwitchOutlined } from '@ant-design/icons';
import AuthWrapper from '../components/AuthWrapper';
import { layoutTokens } from '../theme';
import { MENU_KEYS } from '../types/permissions';
import type { ResourceTreeNode } from '../types';

const IndexPage = lazy(() => import('../pages/IndexPage'));
const LoginPage = lazy(() => import('../pages/LoginPage'));
const AccountListPage = lazy(() => import('../pages/AccountManagement/AccountListPage'));
const CreateAccountPage = lazy(() => import('../pages/AccountManagement/CreateAccountPage'));
const EditAccountPage = lazy(() => import('../pages/AccountManagement/EditAccountPage'));
const ApplicationListPage = lazy(() => import('../pages/ApplicationManagement/ApplicationListPage'));
const CreateApplicationPage = lazy(() => import('../pages/ApplicationManagement/CreateApplicationPage'));
const EditApplicationPage = lazy(() => import('../pages/ApplicationManagement/EditApplicationPage'));
const AuthenticationPolicyListPage = lazy(() => import('../pages/AuthenticationManagement/AuthenticationPolicyListPage'));
const CreateAuthenticationPolicyPage = lazy(() => import('../pages/AuthenticationManagement/CreateAuthenticationPolicyPage'));
const EditAuthenticationPolicyPage = lazy(() => import('../pages/AuthenticationManagement/EditAuthenticationPolicyPage'));
const PermissionListPage = lazy(() => import('../pages/AuthorizationManagement/PermissionListPage'));
const CreatePermissionPage = lazy(() => import('../pages/AuthorizationManagement/CreatePermissionPage'));
const EditPermissionPage = lazy(() => import('../pages/AuthorizationManagement/EditPermissionPage'));
const RoleManagementPage = lazy(() => import('../pages/AuthorizationManagement/RoleManagementPage'));
const CreateRolePage = lazy(() => import('../pages/AuthorizationManagement/CreateRolePage'));
const EditRolePage = lazy(() => import('../pages/AuthorizationManagement/EditRolePage'));
const PermissionSubscriptionPage = lazy(() => import('../pages/AuthorizationManagement/PermissionSubscriptionPage'));
const ResourceManagementPage = lazy(() => import('../pages/AuthorizationManagement/ResourceManagementPage'));

const withSuspense = (element: ReactNode) => (
  <Suspense fallback={<Spin style={{ display: 'block', margin: `${layoutTokens.sectionGap * 4}px auto` }} />}>
    {element}
  </Suspense>
);

type MenuItem = NonNullable<MenuProps['items']>[number] & {
  children?: MenuItem[];
};

export const routes: RouteObject[] = [
  { path: '/login', element: withSuspense(<LoginPage />) },
  {
    path: '/',
    handle: { key: 'iam', icon: <SafetyOutlined />, label: '身份访问管理', breadcrumb: '身份访问管理' },
    element: withSuspense(<AuthWrapper><IndexPage /></AuthWrapper>),
    children: [
      {
        path: 'account_management',
        handle: { key: 'account_management', icon: <UserSwitchOutlined />, label: '账号管理', breadcrumb: '账号管理' },
        children: [
          {
            path: 'account_list',
            handle: {
              key: 'account_list',
              label: <Link to="/account_management/account_list">用户列表</Link>,
              breadcrumb: '用户列表',
              permission: MENU_KEYS.accountList,
            },
            element: withSuspense(<AccountListPage />),
          },
          {
            path: 'create_account',
            handle: { breadcrumb: '创建账号', permission: MENU_KEYS.accountList },
            element: withSuspense(<CreateAccountPage />),
          },
          {
            path: 'edit_account/:id',
            handle: { breadcrumb: '编辑账号', permission: MENU_KEYS.accountList },
            element: withSuspense(<EditAccountPage />),
          },
        ],
      },
      {
        path: 'application_management',
        handle: { key: 'application_management', icon: <AppstoreOutlined />, label: '应用管理', breadcrumb: '应用管理' },
        children: [
          {
            path: 'application_list',
            handle: {
              key: 'application_list',
              label: <Link to="/application_management/application_list">应用列表</Link>,
              breadcrumb: '应用列表',
              permission: MENU_KEYS.applicationList,
            },
            element: withSuspense(<ApplicationListPage />),
          },
          {
            path: 'create_application',
            handle: { breadcrumb: '创建应用', permission: MENU_KEYS.applicationList },
            element: withSuspense(<CreateApplicationPage />),
          },
          {
            path: 'edit_application/:id',
            handle: { breadcrumb: '编辑应用', permission: MENU_KEYS.applicationList },
            element: withSuspense(<EditApplicationPage />),
          },
        ],
      },
      {
        path: 'authentication_management',
        handle: { key: 'authentication_management', icon: <SecurityScanOutlined />, label: '认证管理', breadcrumb: '认证管理' },
        children: [
          {
            path: 'authentication_policy_list',
            handle: {
              key: 'authentication_policy_list',
              label: <Link to="/authentication_management/authentication_policy_list">认证策略</Link>,
              breadcrumb: '认证策略',
            },
            element: withSuspense(<AuthenticationPolicyListPage />),
          },
          {
            path: 'create_authentication_policy/:type',
            handle: { breadcrumb: '创建策略' },
            element: withSuspense(<CreateAuthenticationPolicyPage />),
          },
          {
            path: 'edit_authentication_policy/:type/:id',
            handle: { breadcrumb: '编辑策略' },
            element: withSuspense(<EditAuthenticationPolicyPage />),
          },
        ],
      },
      {
        path: 'authorization_management',
        handle: { key: 'authorization_management', icon: <KeyOutlined />, label: '权限管理', breadcrumb: '权限管理' },
        children: [
          {
            path: 'permission_list',
            handle: {
              key: 'permission_list',
              label: <Link to="/authorization_management/permission_list">权限列表</Link>,
              breadcrumb: '权限列表',
              permission: MENU_KEYS.permissionList,
            },
            element: withSuspense(<PermissionListPage />),
          },
          {
            path: 'permission_create',
            handle: { breadcrumb: '创建权限', permission: MENU_KEYS.permissionList },
            element: withSuspense(<CreatePermissionPage />),
          },
          {
            path: 'permission_edit/:id',
            handle: { breadcrumb: '编辑权限', permission: MENU_KEYS.permissionList },
            element: withSuspense(<EditPermissionPage />),
          },
          {
            path: 'role_management',
            handle: {
              key: 'role_management',
              label: <Link to="/authorization_management/role_management">角色管理</Link>,
              breadcrumb: '角色管理',
              permission: MENU_KEYS.roleManagement,
            },
            element: withSuspense(<RoleManagementPage />),
          },
          {
            path: 'role_create',
            handle: { breadcrumb: '创建角色', permission: MENU_KEYS.roleManagement },
            element: withSuspense(<CreateRolePage />),
          },
          {
            path: 'role_edit/:id',
            handle: { breadcrumb: '编辑角色', permission: MENU_KEYS.roleManagement },
            element: withSuspense(<EditRolePage />),
          },
          {
            path: 'permission_subscription',
            handle: {
              key: 'permission_subscription',
              label: <Link to="/authorization_management/permission_subscription">权限订阅</Link>,
              breadcrumb: '权限订阅',
              permission: MENU_KEYS.permissionSubscription,
            },
            element: withSuspense(<PermissionSubscriptionPage />),
          },
          {
            path: 'resource_management',
            handle: {
              key: 'resource_management',
              label: <Link to="/authorization_management/resource_management">资源管理</Link>,
              breadcrumb: '资源管理',
              permission: MENU_KEYS.resourceManagement,
            },
            element: withSuspense(<ResourceManagementPage />),
          },
        ],
      },
    ],
  },
];

export const router = createBrowserRouter(routes);

export const convertRoutesToMenuItems = (
  routeList: RouteObject[],
  hasPermission: (permission?: string) => boolean,
) : MenuItem[] => routeList
  .map((route): MenuItem | null => {
    const handle = route.handle as { key?: string; icon?: ReactNode; label?: ReactNode } | undefined;
    const key = handle?.key || route.path || '';

    if (route.children && route.children.length > 0) {
      const childMenuItems = convertRoutesToMenuItems(route.children, hasPermission);
      if (childMenuItems.length > 0) {
    return { key, icon: handle?.icon, label: handle?.label, children: childMenuItems };
      }
      return null;
    }

    const permission = (route.handle as { permission?: string } | undefined)?.permission;
    if (!handle?.label || !hasPermission(permission)) return null;
    return { key, icon: handle.icon, label: handle.label };
  })
  .filter((item): item is NonNullable<typeof item> => item !== null);

/** 路由元数据：key → 跳转路径与图标（菜单树节点不含路径/图标，仍取自路由表） */
export interface RouteMeta {
  path: string;
  icon?: ReactNode;
  /** 叶子路由（无 children），可作为菜单点击跳转目标 */
  navigable: boolean;
}

const joinPath = (parent: string, child?: string): string => {
  if (!child) return parent || '/';
  return `${parent.replace(/\/$/, '')}/${child}`.replace(/\/\/+/g, '/');
};

/** 按 handle.key 收集路由元数据，path 按父子拼接为绝对路径 */
export const collectRouteMeta = (routeList: RouteObject[]): Map<string, RouteMeta> => {
  const meta = new Map<string, RouteMeta>();
  const walk = (routes: RouteObject[], parentPath: string) => {
    for (const route of routes) {
      const handle = route.handle as { key?: string; icon?: ReactNode } | undefined;
      const path = joinPath(parentPath, route.path);
      if (handle?.key && !meta.has(handle.key)) {
        meta.set(handle.key, { path, icon: handle.icon, navigable: !route.children?.length });
      }
      if (route.children) walk(route.children, path);
    }
  };
  walk(routeList, '');
  return meta;
};

/**
 * 以服务端菜单树（specs/008-account-app-context US3）直接构建菜单：
 * label 取 resource.name，层级与顺序按服务端返回（display_order 排序），
 * 图标与跳转路径按 resource.key 从路由表元数据补齐；未知键告警并跳过。
 */
export const buildMenuItemsFromTree = (
  nodes: ResourceTreeNode[],
  routeMeta: ReadonlyMap<string, RouteMeta>,
): MenuItem[] => {
  const items: MenuItem[] = [];
  for (const node of nodes) {
    const meta = routeMeta.get(node.resource.key);
    if (!meta) {
      console.warn(`[menu] 菜单树键 "${node.resource.key}" 在路由表中无对应 handle.key，已跳过`);
      continue;
    }
    if (node.children.length > 0) {
      const children = buildMenuItemsFromTree(node.children, routeMeta);
      if (children.length === 0) continue;
      items.push({ key: node.resource.key, icon: meta.icon, label: node.resource.name, children });
    } else {
      items.push({ key: node.resource.key, icon: meta.icon, label: node.resource.name });
    }
  }
  return items;
};
