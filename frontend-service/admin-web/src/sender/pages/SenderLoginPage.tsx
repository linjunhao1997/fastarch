import { createLoginPage } from '@/shared/pages/LoginPage';
import { authApi } from '@/sender/api/sender';

/** sender 控制台独立登录页（ADR 0007）：凭据校验委托 IAM，登录即锁定 sender 应用 */
export default createLoginPage({
  title: 'FastArch 消息中心',
  login: (data) => authApi.login(data),
  consoleKind: 'sender',
  homePath: '/sender',
});