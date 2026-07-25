import {
  AppstoreOutlined,
  KeyOutlined,
  SafetyOutlined,
  SecurityScanOutlined,
  UserSwitchOutlined,
} from "@ant-design/icons";
import { Link } from "react-router";

export const MenuItems = [
  {
    key: "iam",
    icon: <SafetyOutlined />,
    label: "身份访问管理(IAM)",
    children: [
      {
        key: "account_management",
        icon: <UserSwitchOutlined />,
        label: "账号管理",
        children: [
          { key: "user_list", label: <Link to="/account_management/account_list">用户列表</Link> },
          { key: "department", label: "部门架构" },
          { key: "role", label: "岗位角色" },
        ],
      },
      {
        key: "authentication_management",
        icon: <SecurityScanOutlined />,
        label: "认证管理",
        children: [
          { key: "authentication_policy_list", label:<Link to="/authentication_management/authentication_policy_list">认证策略</Link> },
        ],
      },
      {
        key: "permission_management",
        icon: <KeyOutlined />,
        label: "权限管理",
        children: [
          { key: "permission_list", label: <Link to="/authrization_management/permission_list">权限列表</Link> },
          { key: "role_management",  label: <Link to="/authrization_management/role_management">角色管理</Link> },
          { key: "permission_subcription", label: <Link to="/authrization_management/permission_subscription">权限订阅</Link> },
        ],
      },
      {
        key: "app_management",
        icon: <AppstoreOutlined />,
        label: "应用管理",
        children: [
          { key: "application_list", label: <Link to="/application_management/application_list">应用列表</Link> },
        ],
      },
    ],
  },
];
