import { Link, createBrowserRouter } from "react-router-dom";
import type { RouteObject } from "react-router-dom";
import IndexPage from "../pages/IndexPage";
import LoginPage from "../pages/LoginPage";
import AccountListPage from "../pages/AccountManagement/AccountListPage";
import CreateAccountPage from "../pages/AccountManagement/CreateAccountPage";
import EditAccountPage from "../pages/AccountManagement/EditAccountPage";
import ApplicationListPage from "../pages/ApplicationManagement/ApplicationListPage";
import CreateApplicationPage from "../pages/ApplicationManagement/CreateApplicationPage";
import EditApplicationPage from "../pages/ApplicationManagement/EditApplicationPage";
import AuthenticationPolicyListPage from "../pages/AuthenticationManagement/AuthenticationPolicyListPage";
import CreateAuthenticationPolicyPage from "../pages/AuthenticationManagement/CreateAuthenticationPolicyPage";
import EditAuthenticationPolicyPage from "../pages/AuthenticationManagement/EditAuthenticationPolicyPage";
import PermissionListPage from "../pages/AuthorizationManagement/PermissionListPage";
import CreatePermissionPage from "../pages/AuthorizationManagement/CreatePermissionPage";
import EditPermissionPage from "../pages/AuthorizationManagement/EditPermissionPage";
import RoleManagementPage from "../pages/AuthorizationManagement/RoleManagementPage";
import CreateRolePage from "../pages/AuthorizationManagement/CreateRolePage";
import EditRolePage from "../pages/AuthorizationManagement/EditRolePage";
import PermissionSubscriptionPage from "../pages/AuthorizationManagement/PermissionSubscriptionPage";
import AuthWrapper from "../components/AuthWrapper";
import { AppstoreOutlined, KeyOutlined, SafetyOutlined, SecurityScanOutlined, UserSwitchOutlined } from "@ant-design/icons";


export const routes: RouteObject[] =
[
    {
      path: "/login",
      element: <LoginPage />,
    },
    {
      path: "/",
      handle: { key: "iam", icon: <SafetyOutlined />, label: "身份访问管理", breadcrumb: "身份访问管理" },
      element: <AuthWrapper><IndexPage /></AuthWrapper>,
      children: [
        {
          path: "account_management",
          handle: { key: "account_management", icon: <UserSwitchOutlined />, label: "账号管理", breadcrumb: "账号管理" },
          children: [
            {
              path: "account_list",
              handle: {
                key: "account_list",
                label: <Link to="/account_management/account_list">用户列表</Link>,
                breadcrumb: "用户列表",
              },
              element: <AccountListPage />,
            },
            {
              path: "create_account",
              handle: {
                breadcrumb: "创建账号",
              },
              element: <CreateAccountPage />,
            },
            {
              path: "edit_account/:id",
              handle: {
                breadcrumb: "编辑账号",
              },
              element: <EditAccountPage />,
            },
          ],
        },
        {
          path: "application_management",
          handle: { key: "application_management", icon:<AppstoreOutlined/>, label: "应用管理", breadcrumb: "应用管理" },
          children: [
            {
              path: "application_list",
              handle: {
                key: "application_list",
                label: <Link to="/application_management/application_list">应用列表</Link>,
                breadcrumb: "应用列表",
              },
              element: <ApplicationListPage />,
            },
            {
              path: "create_application",
              handle: {
                breadcrumb: "创建应用",
              },
              element: <CreateApplicationPage />,
            },
            {
              path: "edit_application/:id",
              handle: {
                breadcrumb: "编辑应用",
              },
              element: <EditApplicationPage />,
            },
          ],
        },
        {
          path: "authentication_management",
          handle: {key:"authentication_management", icon:<SecurityScanOutlined/>, label:"认证管理", breadcrumb: "认证管理" },
          children: [
            {
              path: "authentication_policy_list",
              handle: {
                key: "authentication_policy_list",
                label: <Link to="/authentication_management/authentication_policy_list">认证策略</Link>,
                breadcrumb: "认证策略",
              },
              element: <AuthenticationPolicyListPage />,
            },
            {
              path: "create_authentication_policy/:type",
              handle: {
                breadcrumb: "创建策略",
              },
              element: <CreateAuthenticationPolicyPage />,
            },
            {
              path: "edit_authentication_policy/:type/:id",
              handle: {
                breadcrumb: "编辑策略",
              },
              element: <EditAuthenticationPolicyPage />,
            },
          ],
        },{
          path: "authorization_management",
          handle: { key:"authorization_management", icon: <KeyOutlined/>,label: "权限管理", breadcrumb: "权限管理" },
          children: [
            {
              path: "permission_list",
              handle: {
                key: "permission_list",
                label: <Link to="/authorization_management/permission_list">权限列表</Link>,
                breadcrumb: "权限列表",
              },
              element: <PermissionListPage />,
            },
            {
              path: "permission_create",
              handle: { breadcrumb: "创建权限" },
              element: <CreatePermissionPage />,
            },
            {
              path: "permission_edit/:id",
              handle: { breadcrumb: "编辑权限" },
              element: <EditPermissionPage />,
            },
            {
              path: "role_management",
              handle: {
                key: "role_management",
                label: <Link to="/authorization_management/role_management">角色管理</Link>,
                breadcrumb: "角色管理",

              },
              element: <RoleManagementPage />,
            },
            {
              path: "role_create",
              handle: { breadcrumb: "创建角色" },
              element: <CreateRolePage />,
            },
            {
              path: "role_edit/:id",
              handle: { breadcrumb: "编辑角色" },
              element: <EditRolePage />,
            },
            {
              path: "permission_subscription",
              handle: {
                key: "permission_subscription",
                label: <Link to="/authorization_management/permission_subscription">权限订阅</Link>,
                breadcrumb: "权限订阅",
              },
              element: <PermissionSubscriptionPage />,
            },
          ],
        },
      ],
    },
  ]

export const router = createBrowserRouter(routes);

/**
 * 将路由配置转换为Ant Design的MenuItems
 * 假设路由配置已经修复了拼写错误
 * @param {Array} routes 路由配置数组
 * @returns {Array} Ant Design的MenuItems数组
 */
export const convertRoutesToMenuItems = (routes: RouteObject[]) => {
    return routes
      .map(route => {
        // 使用handle.key作为key，如果没有则使用path
        const key = route.handle?.key || route.path;
        
        // 如果有子路由，递归处理子路由
        if (route.children && route.children.length > 0) {
          const childMenuItems = convertRoutesToMenuItems(route.children);
          
          // 如果有子菜单项，即使当前route的label不存在，也返回该菜单项（作为父级菜单）
          if (childMenuItems.length > 0) {
            return {
              key,
              icon: route.handle?.icon,
              label: route.handle?.label, // 可能为undefined
              children: childMenuItems,
            };
          }
          
          // 如果没有子菜单项，则不返回该菜单项
          return null;
        }
        
        // 对于叶子节点，只有当label存在时才返回
        if (route.handle?.label) {
          return {
            key,
            icon: route.handle?.icon,
            label: route.handle?.label,
          };
        }
        
        // 如果叶子节点的label不存在，则不返回该菜单项
        return null;
      })
      // 过滤掉返回值为null的项
      .filter((item): item is NonNullable<typeof item> => item !== null);
  };
