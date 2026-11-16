import { Card, Typography } from 'antd';
import { useApplicationContext } from '../context/ApplicationContext';

/** 首页：展示当前应用上下文，引导从左侧菜单进入功能页 */
export default function HomePage() {
  const { currentApplication } = useApplicationContext();
  return (
    <Card>
      <Typography.Title level={4} style={{ marginTop: 0 }}>
        欢迎使用 FastArch 身份认证平台
      </Typography.Title>
      <Typography.Paragraph type="secondary">
        当前应用：{currentApplication ? `${currentApplication.name}（${currentApplication.code}）` : '未选择'}
      </Typography.Paragraph>
      <Typography.Paragraph type="secondary">
        请从左侧菜单进入功能页面；切换顶部应用可管理其他应用的菜单、字典与权限。
      </Typography.Paragraph>
    </Card>
  );
}
