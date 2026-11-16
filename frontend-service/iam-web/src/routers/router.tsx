import { Suspense, lazy, type ReactNode } from 'react';
import { Link, createBrowserRouter } from 'react-router-dom';
import type { RouteObject } from 'react-router-dom';
import { Spin } from 'antd';
import type { MenuProps } from 'antd';
import {
  ApiOutlined,
  AppstoreOutlined,
  BookOutlined,
  KeyOutlined,
  LockOutlined,
  MenuOutlined,
  SafetyOutlined,
  SecurityScanOutlined,
  TeamOutlined,
  UnorderedListOutlined,
  UserOutlined,
  UserSwitchOutlined,
} from '@ant-design/icons';
import AuthWrapper from '../components/AuthWrapper';
import { layoutTokens } from '../theme';
import { PERMISSIONS } from '../types/permissions';
import type { MenuTreeNode } from '../types';

const IndexPage = lazy(() => import('../pages/IndexPage'));
const LoginPage = lazy(() => import('../pages/LoginPage'));
const HomePage = lazy(() => import('../pages/HomePage'));
const AccountListPage = lazy(() => import('../pages/AccountManagement/AccountListPage'));
const CreateAccountPage = lazy(() => import('../pages/AccountManagement/CreateAccountPage'));
const EditAccountPage = lazy(() => import('../pages/AccountManagement/EditAccountPage'));
const ApplicationListPage = lazy(() => import('../pages/ApplicationManagement/ApplicationListPage'));
const CreateApplicationPage = lazy(() => import('../pages/ApplicationManagement/CreateApplicationPage'));
const EditApplicationPage = lazy(() => import('../pages/ApplicationManagement/EditApplicationPage'));
const AuthenticationPolicyListPage = lazy(() => import('../pages/AuthenticationManagement/AuthenticationPolicyListPage'));
const CreateAuthenticationPolicyPage = lazy(() => import('../pages/AuthenticationManagement/CreateAuthenticationPolicyPage'));
const EditAuthenticationPolicyPage = lazy(() => import('../pages/AuthenticationManagement/EditAuthenticationPolicyPage'));
const AccessCredentialListPage = lazy(() => import('../pages/AuthenticationManagement/AccessCredentialListPage'));
const PermissionListPage = lazy(() => import('../pages/AuthorizationManagement/PermissionListPage'));
const CreatePermissionPage = lazy(() => import('../pages/AuthorizationManagement/CreatePermissionPage'));
const EditPermissionPage = lazy(() => import('../pages/AuthorizationManagement/EditPermissionPage'));
const RoleManagementPage = lazy(() => import('../pages/AuthorizationManagement/RoleManagementPage'));
const CreateRolePage = lazy(() => import('../pages/AuthorizationManagement/CreateRolePage'));
const EditRolePage = lazy(() => import('../pages/AuthorizationManagement/EditRolePage'));
const MenuManagementPage = lazy(() => import('../pages/AuthorizationManagement/MenuManagementPage'));
const DictManagementPage = lazy(() => import('../pages/AuthorizationManagement/DictManagementPage'));

const withSuspense = (element: ReactNode) => (
  <Suspense fallback={<Spin style={{ display: 'block', margin: `${layoutTokens.sectionGap * 4}px auto` }} />}>
    {element}
  </Suspense>
);

type MenuItem = NonNullable<MenuProps['items']>[number] & {
  children?: MenuItem[];
};

/**
 * 路由表：叶子 handle.permission 声明访问所需的权限码（词汇 = 后端
 * permission_codes.go PERMISSION_CODES），直访守卫据此判定；
 * 菜单可见性另由服务端 my-menu-tree（角色菜单绑定）决定，路由表仅作兜底。
 */
export const routes: RouteObject[] = [
  { path: '/login', element: withSuspense(<LoginPage />) },
  {
    path: '/',
    handle: { label: '身份访问管理', breadcrumb: '身份访问管理' },
    element: withSuspense(<AuthWrapper><IndexPage /></AuthWrapper>),
    children: [
      {
        index: true,
        handle: { breadcrumb: '首页' },
        element: withSuspense(<HomePage />),
      },
      {
        path: 'account_management',
        handle: { label: '用户管理', breadcrumb: '用户管理' },
        children: [
          {
            path: 'account_list',
            handle: {
              label: <Link to="/account_management/account_list">用户列表</Link>,
              breadcrumb: '用户列表',
              permission: PERMISSIONS.userList,
            },
            element: withSuspense(<AccountListPage />),
          },
          {
            path: 'create_account',
            handle: { breadcrumb: '创建用户', permission: PERMISSIONS.userList },
            element: withSuspense(<CreateAccountPage />),
          },
          {
            path: 'edit_account/:id',
            handle: { breadcrumb: '编辑用户', permission: PERMISSIONS.userList },
            element: withSuspense(<EditAccountPage />),
          },
        ],
      },
      {
        path: 'application_management',
        handle: { label: '应用管理', breadcrumb: '应用管理' },
        children: [
          {
            path: 'application_list',
            handle: {
              label: <Link to="/application_management/application_list">应用列表</Link>,
              breadcrumb: '应用列表',
              permission: PERMISSIONS.applicationList,
            },
            element: withSuspense(<ApplicationListPage />),
          },
          {
            path: 'create_application',
            handle: { breadcrumb: '创建应用', permission: PERMISSIONS.applicationList },
            element: withSuspense(<CreateApplicationPage />),
          },
          {
            path: 'edit_application/:id',
            handle: { breadcrumb: '编辑应用', permission: PERMISSIONS.applicationList },
            element: withSuspense(<EditApplicationPage />),
          },
        ],
      },
      {
        path: 'authentication_management',
        handle: { label: '认证管理', breadcrumb: '认证管理' },
        children: [
          {
            path: 'authentication_policy_list',
            handle: {
              label: <Link to="/authentication_management/authentication_policy_list">认证策略</Link>,
              breadcrumb: '认证策略',
            },
            element: withSuspense(<AuthenticationPolicyListPage />),
          },
          {
            path: 'access_credential_list',
            handle: {
              label: <Link to="/authentication_management/access_credential_list">访问凭证</Link>,
              breadcrumb: '访问凭证',
              permission: PERMISSIONS.credentialList,
            },
            element: withSuspense(<AccessCredentialListPage />),
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
        handle: { label: '权限管理', breadcrumb: '权限管理' },
        children: [
          {
            path: 'permission_list',
            handle: {
              label: <Link to="/authorization_management/permission_list">权限列表</Link>,
              breadcrumb: '权限列表',
              permission: PERMISSIONS.permissionList,
            },
            element: withSuspense(<PermissionListPage />),
          },
          {
            path: 'permission_create',
            handle: { breadcrumb: '创建权限', permission: PERMISSIONS.permissionList },
            element: withSuspense(<CreatePermissionPage />),
          },
          {
            path: 'permission_edit/:id',
            handle: { breadcrumb: '编辑权限', permission: PERMISSIONS.permissionList },
            element: withSuspense(<EditPermissionPage />),
          },
          {
            path: 'role_management',
            handle: {
              label: <Link to="/authorization_management/role_management">角色管理</Link>,
              breadcrumb: '角色管理',
              permission: PERMISSIONS.roleList,
            },
            element: withSuspense(<RoleManagementPage />),
          },
          {
            path: 'role_create',
            handle: { breadcrumb: '创建角色', permission: PERMISSIONS.roleList },
            element: withSuspense(<CreateRolePage />),
          },
          {
            path: 'role_edit/:id',
            handle: { breadcrumb: '编辑角色', permission: PERMISSIONS.roleList },
            element: withSuspense(<EditRolePage />),
          },
          {
            path: 'menu_management',
            handle: {
              label: <Link to="/authorization_management/menu_management">菜单管理</Link>,
              breadcrumb: '菜单管理',
              permission: PERMISSIONS.menuList,
            },
            element: withSuspense(<MenuManagementPage />),
          },
          {
            path: 'dict_management',
            handle: {
              label: <Link to="/authorization_management/dict_management">字典管理</Link>,
              breadcrumb: '字典管理',
              permission: PERMISSIONS.dictList,
            },
            element: withSuspense(<DictManagementPage />),
          },
        ],
      },
    ],
  },
];

export const router = createBrowserRouter(routes);

/** 菜单树 icon 字段（AntD 图标名）→ 图标组件（t_menu.icon 的取值域） */
export const MENU_ICONS: Record<string, ReactNode> = {
  SafetyOutlined: <SafetyOutlined />,
  UserOutlined: <UserOutlined />,
  UserSwitchOutlined: <UserSwitchOutlined />,
  AppstoreOutlined: <AppstoreOutlined />,
  SecurityScanOutlined: <SecurityScanOutlined />,
  LockOutlined: <LockOutlined />,
  KeyOutlined: <KeyOutlined />,
  ApiOutlined: <ApiOutlined />,
  MenuOutlined: <MenuOutlined />,
  BookOutlined: <BookOutlined />,
  UnorderedListOutlined: <UnorderedListOutlined />,
  TeamOutlined: <TeamOutlined />,
};

/**
 * 路由元数据：绝对 path → 面包屑/权限码等。菜单渲染仅按 path 对齐，
 * path 不在路由表中的菜单节点（如未开发的页面）会被跳过并告警。
 */
export interface RouteMeta {
  /** 权限码（直访守卫使用） */
  permission?: string;
}

const joinPath = (parent: string, child?: string): string => {
  if (!child) return parent || '/';
  return `${parent.replace(/\/$/, '')}/${child}`.replace(/\/\/+/g, '/');
};

/** 遍历路由表收集 path → meta（仅叶子路由有意义） */
export const collectRouteMeta = (routeList: RouteObject[]): Map<string, RouteMeta> => {
  const meta = new Map<string, RouteMeta>();
  const walk = (routeList: RouteObject[], parentPath: string) => {
    for (const route of routeList) {
      const path = joinPath(parentPath, route.path);
      const handle = route.handle as { permission?: string } | undefined;
      if (!route.children?.length && handle?.permission) meta.set(path, { permission: handle.permission });
      if (route.children) walk(route.children, path);
    }
  };
  walk(routeList, '');
  return meta;
};

/**
 * 以服务端菜单树（my-menu-tree，t_menu 语义）直接构建左侧菜单：
 * 层级/名称/图标/顺序均取自服务端（结构与展示归管理员维护），
 * 跳转按 menu.path 匹配路由表；未知 path 告警并跳过（页面未开发时隐藏入口）。
 */
export const buildMenuItemsFromTree = (
  nodes: MenuTreeNode[],
  routeMeta: ReadonlyMap<string, RouteMeta>,
): MenuItem[] => {
  const items: MenuItem[] = [];
  for (const node of nodes) {
    const icon = MENU_ICONS[node.menu.icon];
    if (node.children.length > 0) {
      const children = buildMenuItemsFromTree(node.children, routeMeta);
      if (children.length === 0) continue;
      items.push({ key: `dir-${node.menu.id}`, icon, label: node.menu.name, children });
    } else {
      const meta = routeMeta.get(node.menu.path);
      if (!meta) {
        console.warn(`[menu] 菜单 "${node.menu.name}" 的路径 "${node.menu.path}" 在路由表中无对应页面，已跳过`);
        continue;
      }
      items.push({ key: node.menu.path, icon, label: node.menu.name });
    }
  }
  return items;
};

/** 收集菜单树全部叶子 path（选中态高亮与直访守卫的合法路径集合） */
export const collectMenuLeafPaths = (nodes: MenuTreeNode[], out: string[] = []): string[] => {
  for (const node of nodes) {
    if (node.children.length > 0) collectMenuLeafPaths(node.children, out);
    else out.push(node.menu.path);
  }
  return out;
};
