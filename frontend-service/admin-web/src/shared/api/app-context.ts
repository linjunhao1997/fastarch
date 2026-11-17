/**
 * 当前应用 code 的读取点（跨 services 共享）。
 *
 * 背景：管理员在宿主切换应用后，菜单/字典的配置要写到对应服务的库——
 * iam 应用走 /iam/...，sender 应用走 /sender/...。API 前缀由这里决定。
 *
 * 时序约束：当前 app code 由 ApplicationContext 在服务端绑定成功后设置
 * （setCurrentAppCode），菜单树请求以 currentAppCode 门控，确保发请求前 code 已就位。
 */
let currentAppCode: string | null = null;

export function getCurrentAppCode(): string | null {
  return currentAppCode;
}

export function setCurrentAppCode(code: string | null): void {
  currentAppCode = code;
}

/**
 * 按当前应用拼菜单/字典 API 前缀。
 * 未设置 code 时回退 'iam'（首屏兜底；正常时序下由门控避免）。
 */
export function appApiPrefix(): string {
  return `/${currentAppCode ?? 'iam'}`;
}
