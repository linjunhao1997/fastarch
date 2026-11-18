import { Card, Typography } from 'antd';
import { useConsole } from '@/shared/context/ConsoleContext';

/** 首页：展示当前控制台（登录即锁定，ADR 0007），引导从左侧菜单进入功能页 */
export default function HomePage() {
  const { label } = useConsole();
  return (
    <Card>
      <Typography.Title level={4} style={{ marginTop: 0 }}>
        欢迎使用{label}
      </Typography.Title>
      <Typography.Paragraph type="secondary">
        当前控制台：{label}
      </Typography.Paragraph>
      <Typography.Paragraph type="secondary">
        请从左侧菜单进入功能页面。
      </Typography.Paragraph>
    </Card>
  );
}