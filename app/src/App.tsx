import { router } from "./routers/router";
import { RouterProvider } from "react-router-dom";
import { ConfigProvider } from "antd";
import zhCN from "antd/locale/zh_CN";
import { NotificationProvider } from "./hooks/useNotification";

export default function App() {
  return (
    <ConfigProvider locale={zhCN}>
      <NotificationProvider>
        <RouterProvider router={router} />
      </NotificationProvider>
    </ConfigProvider>
  );
}