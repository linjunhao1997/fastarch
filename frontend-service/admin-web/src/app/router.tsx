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
import AuthWrapper from '@/shared/components/AuthWrapper';
import { layoutTokens } from '@/shared/theme';
import { PERMISSIONS } from '@/shared/types/permissions';
import type { MenuTreeNode } from '@/shared/types';
import { senderRoutes } from '@/sender/router';

const IndexPage = lazy(() => import('@/app/IndexPage'));
const LoginPage = lazy(() => import('@/shared/pages/LoginPage'));
const SenderLoginPage = lazy(() => import('@/sender/pages/SenderLoginPage'));
const HomePage = lazy(() => import('@/app/HomePage'));
const AccountListPage = lazy(() => import('@/iam/pages/AccountManagement/AccountListPage'));
const CreateAccountPage = lazy(() => import('@/iam/pages/AccountManagement/CreateAccountPage'));
const EditAccountPage = lazy(() => import('@/iam/pages/AccountManagement/EditAccountPage'));
const ApplicationListPage = lazy(() => import('@/iam/pages/ApplicationManagement/ApplicationListPage'));
const CreateApplicationPage = lazy(() => import('@/iam/pages/ApplicationManagement/CreateApplicationPage'));
const EditApplicationPage = lazy(() => import('@/iam/pages/ApplicationManagement/EditApplicationPage'));
const AuthenticationPolicyListPage = lazy(() => import('@/iam/pages/AuthenticationManagement/AuthenticationPolicyListPage'));
const CreateAuthenticationPolicyPage = lazy(() => import('@/iam/pages/AuthenticationManagement/CreateAuthenticationPolicyPage'));
const EditAuthenticationPolicyPage = lazy(() => import('@/iam/pages/AuthenticationManagement/EditAuthenticationPolicyPage'));
const AccessCredentialListPage = lazy(() => import('@/iam/pages/AuthenticationManagement/AccessCredentialListPage'));
const PermissionListPage = lazy(() => import('@/iam/pages/AuthorizationManagement/PermissionListPage'));
const CreatePermissionPage = lazy(() => import('@/iam/pages/AuthorizationManagement/CreatePermissionPage'));
const EditPermissionPage = lazy(() => import('@/iam/pages/AuthorizationManagement/EditPermissionPage'));
const RoleManagementPage = lazy(() => import('@/iam/pages/AuthorizationManagement/RoleManagementPage'));
const CreateRolePage = lazy(() => import('@/iam/pages/AuthorizationManagement/CreateRolePage'));
const EditRolePage = lazy(() => import('@/iam/pages/AuthorizationManagement/EditRolePage'));
const MenuManagementPage = lazy(() => import('@/shared/pages/MenuManagementPage'));
const DictManagementPage = lazy(() => import('@/shared/pages/DictManagementPage'));

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
 * 菜单可见性另由前端按 my-menu-tree + 权限码门槛决定，路由表仅作兜底。
 */
export const routes: RouteObject[] = [
  { path: '/login', element: withSuspense(<LoginPage />) },
  { path: '/sender/login', element: withSuspense(<SenderLoginPage />) },
  {
    path: '/',
    element: withSuspense(<AuthWrapper consoleKind="iam"><IndexPage /></AuthWrapper>),
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
  // sender 控制台（ADR 0007 单工程双控制台）：独立登录页 + 独立子树
  {
    path: '/sender',
    element: withSuspense(<AuthWrapper consoleKind="sender"><IndexPage /></AuthWrapper>),
    children: [...senderRoutes],
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
 *
 * 展示门槛（ADR 0002）：服务端已按角色绑定过滤基础可见性；此处再按菜单声明的
 * permission_code 与用户持有码（heldCodes）二次过滤——声明了码且用户未持有则隐藏。
 * 纯 UX 显隐，非安全边界（API 访问仍由服务端 fail-closed 判码）。
 */
export const buildMenuItemsFromTree = (
  nodes: MenuTreeNode[],
  routeMeta: ReadonlyMap<string, RouteMeta>,
  heldCodes: ReadonlySet<string>,
): MenuItem[] => {
  const items: MenuItem[] = [];
  for (const node of nodes) {
    const icon = MENU_ICONS[node.menu.icon];
    if (node.children.length > 0) {
      const children = buildMenuItemsFromTree(node.children, routeMeta, heldCodes);
      if (children.length === 0) continue;
      items.push({ key: `dir-${node.menu.id}`, icon, label: node.menu.name, children });
    } else {
      if (node.menu.permissionCode && !heldCodes.has(node.menu.permissionCode)) continue;
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
