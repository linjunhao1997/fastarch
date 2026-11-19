import { useEffect, type ReactNode } from 'react';
import { Card, Col, Row, Typography } from 'antd';
import {
  ApiOutlined,
  MessageOutlined,
  SafetyOutlined,
} from '@ant-design/icons';
import { Navigate, useNavigate } from 'react-router-dom';
import { LOGIN_PATH } from '@/app/routes';
import { CONSOLE_REGISTRY, setConsole, type ConsoleKinds } from '@/shared/api/app-context';
import { getStoredToken } from '@/shared/api/request';
import { layoutTokens, themeTokens } from '@/shared/theme';

const CONSOLE_ICONS: Record<ConsoleKinds, ReactNode> = {
  iam: <SafetyOutlined />,
  sender: <MessageOutlined />,
};

/**
 * 统一登录后的控制台选择页。
 *
 * 这里选的是「控制台」（决定菜单/字典与 API 前缀），不是「数据范围」。
 * 数据范围由用户接入集合决定，进入控制台后在左上角下拉切换。
 * 因此即使某用户只接入了 sender 应用，仍可进入 IAM 控制台查看 sender 的数据。
 */
export default function ConsoleSelectPage() {
  const navigate = useNavigate();

  // 本页位于受保护区之外，需要自行校验登录态。
  useEffect(() => {
    const token = getStoredToken();
    if (!token || token.trim().length === 0) navigate(LOGIN_PATH, { replace: true });
  }, [navigate]);

  if (!getStoredToken()) return <Navigate to={LOGIN_PATH} replace />;

  return (
    <div
      style={{
        minHeight: '100vh',
        padding: `${layoutTokens.sectionGap * 6}px ${layoutTokens.contentPadding}px`,
        background: `
          radial-gradient(circle at 20% 0%, rgba(37, 99, 235, 0.10) 0%, transparent 45%),
          ${layoutTokens.backgroundGradient}
        `,
      }}
    >
      <div style={{ maxWidth: 880, margin: '0 auto' }}>
        <Typography.Title level={2} style={{ textAlign: 'center', marginBottom: 8 }}>
          选择要进入的控制台
        </Typography.Title>
        <Typography.Paragraph
          type="secondary"
          style={{ textAlign: 'center', marginBottom: layoutTokens.sectionGap * 5 }}
        >
          控制台决定可以使用的功能菜单；进入后可在左上角切换要操作的数据范围。
        </Typography.Paragraph>

        <Row gutter={[layoutTokens.sectionGap * 2, layoutTokens.sectionGap * 2]}>
          {CONSOLE_REGISTRY.map((consoleItem) => (
            <Col key={consoleItem.code} xs={24} sm={12}>
              <Card
                hoverable
                onClick={() => {
                  // 锁定控制台：菜单/字典前缀自此固定到该服务。
                  setConsole(consoleItem.code);
                  navigate(consoleItem.path);
                }}
                style={{ height: '100%', textAlign: 'center' }}
              >
                <div style={{ fontSize: 40, color: themeTokens.colorPrimary, margin: '12px 0 20px' }}>
                  {CONSOLE_ICONS[consoleItem.code] ?? <ApiOutlined />}
                </div>
                <Card.Meta title={consoleItem.label} description={consoleItem.description} />
              </Card>
            </Col>
          ))}
        </Row>
      </div>
    </div>
  );
}
