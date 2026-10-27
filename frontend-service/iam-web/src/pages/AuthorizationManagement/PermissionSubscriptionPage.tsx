import { useMemo } from 'react';
import { Button, Flex, Tag, Tooltip } from 'antd';
import { CheckCircleOutlined, ClockCircleOutlined, CloseCircleOutlined } from '@ant-design/icons';
import { useSearchParams } from 'react-router';
import { subscriptionApi } from '../../api/iam';
import { ListPageShell } from '../../components/ListPageShell';
import { useTableQuery } from '../../hooks/useTableQuery';
import type { Subscription } from '../../types';
import { ACTIONS } from '../../types/copywriting';

const STATUS_CONFIG: Record<string, { color: string; icon: React.ReactNode; label: string }> = {
  approved: { color: 'success', icon: <CheckCircleOutlined />, label: '已通过' },
  rejected: { color: 'error', icon: <CloseCircleOutlined />, label: '已拒绝' },
  pending: { color: 'warning', icon: <ClockCircleOutlined />, label: '待审批' },
};

export default function PermissionSubscriptionPage() {
  const [searchParams] = useSearchParams();
  const tableQuery = useTableQuery({
    domain: 'subscriptions',
    fetcher: (currentPage, pageSize, keyword) => subscriptionApi.list(currentPage, pageSize, keyword),
    getItems: (response) => response.subscriptions,
  });

  const columns = useMemo(() => [
    { title: 'ID', dataIndex: 'id', key: 'id', width: 80, hidden: true },
    { title: '源应用', dataIndex: 'sourceApplicationCode', key: 'sourceApplicationCode', width: 200, ellipsis: true },
    { title: '目标应用', dataIndex: 'targetApplicationCode', key: 'targetApplicationCode', width: 200, ellipsis: true },
    {
      title: '角色编码',
      dataIndex: 'roleCodeList',
      key: 'roleCodeList',
      width: 200,
      render: (roleCodeList: string[] | undefined) =>
        roleCodeList && roleCodeList.length > 0
          ? <Flex gap={4} wrap>{roleCodeList.slice(0, 2).map((code) => <Tag key={code}>{code}</Tag>)}</Flex>
          : '—',
    },
    {
      title: '权限编码',
      dataIndex: 'permissionCodeList',
      key: 'permissionCodeList',
      width: 200,
      render: (permissionCodeList: string[] | undefined) =>
        permissionCodeList && permissionCodeList.length > 0
          ? <Flex gap={4} wrap>{permissionCodeList.slice(0, 2).map((code) => <Tag key={code}>{code}</Tag>)}</Flex>
          : '—',
    },
    { title: '申请人', dataIndex: 'applicantName', key: 'applicantName', width: 100 },
    { title: '审批人', dataIndex: 'approverName', key: 'approverName', width: 100 },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (status: string) => {
        const config = STATUS_CONFIG[status];
        if (!config) return <Tag>{status}</Tag>;
        return <Tag icon={config.icon} color={config.color}>{config.label}</Tag>;
      },
    },
  ], []);

  return (
    <ListPageShell<Subscription>
      columns={columns}
      tableQuery={tableQuery}
      searchPlaceholder="搜索源应用或目标应用"
      keyword={searchParams.get('keyword') ?? ''}
      onSearch={tableQuery.setKeyword}
      emptyDescription="暂无订阅记录"
      extraActions={(
        <Tooltip title="功能开发中">
          <Button disabled>{`${ACTIONS.create}订阅`}</Button>
        </Tooltip>
      )}
    />
  );
}