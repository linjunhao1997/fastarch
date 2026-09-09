import { useMemo } from 'react';
import { Button, Flex, Tag, Tooltip } from 'antd';
import { CheckCircleOutlined, ClockCircleOutlined, CloseCircleOutlined } from '@ant-design/icons';
import { useSearchParams } from 'react-router';
import { subscriptionApi } from '../../api/iam';
import { ListPageShell } from '../../components/ListPageShell';
import { useTableQuery } from '../../hooks/useTableQuery';
import type { Subscription } from '../../types';
import { ACTIONS } from '../../types/copywriting';

export default function PermissionSubscriptionPage() {
  const [searchParams] = useSearchParams();
  const tableQuery = useTableQuery({
    domain: 'subscriptions',
    fetcher: (currentPage, pageSize, keyword) => subscriptionApi.list(currentPage, pageSize, keyword),
    getItems: (response) => response.subscriptions,
  });

  const columns = useMemo(() => [
    { title: 'ID', dataIndex: 'id', key: 'id' },
    { title: '源应用', dataIndex: 'sourceApplicationCode', key: 'sourceApplicationCode' },
    { title: '目标应用', dataIndex: 'targetApplicationCode', key: 'targetApplicationCode' },
    {
      title: '角色编码',
      dataIndex: 'roleCodeList',
      key: 'roleCodeList',
      render: (roleCodeList: string[] | undefined) =>
        roleCodeList && roleCodeList.length > 0
          ? <Flex gap={4} wrap>{roleCodeList.map((code) => <Tag key={code}>{code}</Tag>)}</Flex>
          : '—',
    },
    {
      title: '权限编码',
      dataIndex: 'permissionCodeList',
      key: 'permissionCodeList',
      render: (permissionCodeList: string[] | undefined) =>
        permissionCodeList && permissionCodeList.length > 0
          ? <Flex gap={4} wrap>{permissionCodeList.map((code) => <Tag key={code}>{code}</Tag>)}</Flex>
          : '—',
    },
    { title: '申请人', dataIndex: 'applicantName', key: 'applicantName' },
    { title: '审批人', dataIndex: 'approverName', key: 'approverName' },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status: string) => {
        if (status === 'approved') return <Tag icon={<CheckCircleOutlined />}>已通过</Tag>;
        if (status === 'rejected') return <Tag icon={<CloseCircleOutlined />}>已拒绝</Tag>;
        return <Tag icon={<ClockCircleOutlined />}>待审批</Tag>;
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
        <Tooltip title="未开放">
          <Button disabled>{`${ACTIONS.create}订阅`}</Button>
        </Tooltip>
      )}
    />
  );
}
