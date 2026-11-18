/**
 * 当前控制台的读取点（登录即锁定，ADR 0007）。
 *
 * 背景：管理员经各自应用的登录页登录后，应用上下文（application_code）即锁定，
 * 无运行时切换。菜单/字典 API 前缀由此决定：iam 应用走 /iam/...，sender 应用走
 * /sender/...。
 *
 * 时序约束：登录成功后由登录页调用 setConsole，控制台内所有请求据此拼前缀。
 */
export type ConsoleKinds = 'iam' | 'sender';

const CONSOLE_STORAGE_KEY = 'app_console';

export const CONSOLE_LABELS: Record<ConsoleKinds, string> = {
  iam: '身份认证平台',
  sender: '消息中心',
};

let currentConsole: ConsoleKinds;

function loadInitialConsole(): ConsoleKinds {
  try {
    const stored = localStorage.getItem(CONSOLE_STORAGE_KEY);
    if (stored === 'iam' || stored === 'sender') return stored;
  } catch {
    // ignore
  }
  return 'iam'; // 缺省：IAM 自有控制台
}
currentConsole = loadInitialConsole();

export function getConsole(): ConsoleKinds {
  return currentConsole;
}

export function setConsole(code: ConsoleKinds): void {
  currentConsole = code;
  try {
    localStorage.setItem(CONSOLE_STORAGE_KEY, code);
  } catch {
    // ignore
  }
}

export function clearConsole(): void {
  try {
    localStorage.removeItem(CONSOLE_STORAGE_KEY);
  } catch {
    // ignore
  }
}

/**
 * 按当前控制台拼菜单/字典 API 前缀。
 * 登录即锁定，取值只有 /iam 或 /sender。
 */
export function appApiPrefix(): string {
  return `/${currentConsole}`;
}