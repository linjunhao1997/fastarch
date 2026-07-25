import { useState } from "react";
import { Layout, Menu, Dropdown, Avatar, Button } from "antd";
import {
  MenuUnfoldOutlined,
  MenuFoldOutlined,
  UserOutlined,
} from "@ant-design/icons";
import { MenuItems } from "../routers";
import { Outlet } from "react-router";
import AutoBreadcrumb from "../components/AutoBreadcrumb";
import { convertRoutesToMenuItems, routes } from "../routers/router";
const { Header, Sider, Content, Footer } = Layout;

export default function IndexPage() {
  const menuItems = convertRoutesToMenuItems(routes)
  const [collapsed, setCollapsed] = useState(false);

  return (
    <Layout style={{ height: "100vh", display: "flex" }}>
      {/* 左侧导航栏 */}
      <Sider
        collapsible
        collapsed={collapsed}
        trigger={null}
        width={220}
        style={{
          background: "#ffffff",
          borderRight: "1px solid #f0f0f0",
          position: "relative",
          boxShadow: "2px 0 8px rgba(28, 36, 56, 0.08)",
        }}
      >
        {/* 折叠按钮（右侧悬浮） */}
        <div
          style={{
            position: "absolute",
            right: collapsed ? -40 : -48,
            top: 18,
            zIndex: 1,
            transition: "all 0.2s",
            background: "#ffffff",
            borderRadius: "50%",
            boxShadow: "0 2px 8px rgba(28, 36, 56, 0.1)",
          }}
        >
          <Button
            type="text"
            icon={collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
            onClick={() => setCollapsed(!collapsed)}
            style={{
              fontSize: 16,
              width: 40,
              height: 40,
              color: "#1890ff",
            }}
          />
        </div>

        {/* 系统LOGO */}
        <div
          style={{
            height: 64,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            fontSize: collapsed ? 22 : 18,
            fontWeight: 600,
            color: "#1890ff",
            borderBottom: "1px solid #f0f0f0",
          }}
        >
          {collapsed ? "⚡" : "智能运营平台"}
        </div>

        {/* 导航菜单 */}
        <Menu
          mode="inline"
          defaultSelectedKeys={["dashboard"]}
          items={menuItems}
          style={{
            height: "calc(100vh - 64px)",
            borderRight: 0,
            padding: "8px 0",
          }}
        />
      </Sider>

      {/* 右侧内容区域 */}
      <Layout>
        {/* 顶部导航栏 */}
        <Header
          style={{
            padding: "0 24px",
            background: "#ffffff",
            display: "flex",
            justifyContent: "flex-end",
            alignItems: "center",
            boxShadow: "0 2px 8px rgba(28, 36, 56, 0.08)",
          }}
        >
          <Dropdown
            menu={{
              items: [
                { key: "profile", label: "个人中心" },
                { key: "settings", label: "系统设置" },
                { type: "divider" },
                { key: "logout", label: "退出登录" },
              ],
            }}
          >
            <div
              style={{
                display: "flex",
                alignItems: "center",
                cursor: "pointer",
              }}
            >
              <Avatar
                style={{
                  backgroundColor: "#1890ff",
                  verticalAlign: "middle",
                }}
                icon={<UserOutlined />}
              />
              <span
                style={{
                  marginLeft: 12,
                  fontWeight: 500,
                  color: "rgba(0, 0, 0, 0.88)",
                }}
              >
                管理员
              </span>
            </div>
          </Dropdown>
        </Header>

        <AutoBreadcrumb style={{ padding: 8 }} />

        {/* 内容区域 */}
        <Content
          style={{
            padding: 8,
            overflow: "auto",
            background: "#f8f9fa",
          }}
        >
          <Outlet />
        </Content>
        <Footer
          style={{
            height: "50px",
            width: "100%",
            display: "flex",
            justifyContent: "center",
            alignItems: "center",
          }}
        >
          Ant Design ©{new Date().getFullYear()} Created by Ant UED
        </Footer>
      </Layout>
    </Layout>
  );
}
