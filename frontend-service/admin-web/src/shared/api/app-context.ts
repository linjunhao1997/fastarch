/**
 * 当前控制台的读取点。
 *
 * 统一登录后由「控制台选择页」或受保护路由子树锁定 console（iam/sender），
 * 控制台内不再切换。菜单/字典 API 前缀由此决定：iam 走 /iam/...，sender 走
 * /sender/...。数据范围（appCode）与控制台正交，见 DataScopeContext。
 */
export type ConsoleKinds = 'iam' | 'sender';

const CONSOLE_STORAGE_KEY = 'app_console';

export const CONSOLE_LABELS: Record<ConsoleKinds, string> = {
  iam: '身份认证平台',
  sender: '消息中心',
};

export interface ConsoleDefinition {
  code: ConsoleKinds;
  label: string;
  /** 控制台根路由（登录后选择控制面板时跳转） */
  path: string;
  description: string;
}

/**
 * 可进入的控制台清单。控制台是编译期概念（路由子树 + 前缀），新增控制台必须同时
 * 登记路由与本表；因此此处作为单一事实源，供「登录后选择控制台」页渲染。
 * IAM 中注册的应用决定的是「数据范围」，不是「控制台」，两者不要混用。
 */
export const CONSOLE_REGISTRY: readonly ConsoleDefinition[] = [
  { code: 'iam', label: CONSOLE_LABELS.iam, path: '/', description: '用户、应用、认证与权限管理' },
  { code: 'sender', label: CONSOLE_LABELS.sender, path: '/sender', description: '消息模板与发送记录' },
] as const;

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