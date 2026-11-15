import dayjs from 'dayjs';

/** 全站统一时间展示格式（protojson google.protobuf.Timestamp 下发 ISO 8601，前端本地化渲染） */
export const TIME_FORMAT = 'YYYY-MM-DD HH:mm:ss';

/**
 * 时间戳字符串 → 本地格式化展示。
 * 空值（null/undefined/''）返回占位符 '—'；解析失败原样返回便于排查脏数据。
 */
export function formatDateTime(value: string | null | undefined): string {
  if (!value) return '—';
  const parsed = dayjs(value);
  return parsed.isValid() ? parsed.format(TIME_FORMAT) : value;
}
