/**
 * 成员与角色：把"谁能担任哪些角色"这件事落成一个可见、可改的页面。
 * 规则：工作角色决定一个人能看到哪些任务队列；
 * 只有当前角色是老板（boss）的人能改，其他人只能看（服务端同样会拦，业务码 3001018）。
 */
import { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  Alert,
  App,
  Button,
  Card,
  Checkbox,
  Empty,
  Modal,
  Space,
  Table,
  Tag,
  Typography,
} from 'antd';
import type { TableColumnsType } from 'antd';
import { memberApi } from '@/flow/api/flow';
import type { MemberBrief } from '@/flow/types/flow';

const { Text, Paragraph } = Typography;

export default function MembersPage() {
  const qc = useQueryClient();
  const { message } = App.useApp();
  const [editing, setEditing] = useState<MemberBrief | null>(null);
  const [picked, setPicked] = useState<string[]>([]);

  const membersQuery = useQuery({
    queryKey: ['flow', 'members'],
    queryFn: () => memberApi.listMembers({ pageSize: 50 }),
  });
  const data = membersQuery.data;
  const isBoss = data?.currentRoleCode === 'boss';

  const allRoles = useMemo(() => data?.allRoles ?? [], [data?.allRoles]);

  const saveMutation = useMutation({
    mutationFn: ({ id, codes }: { id: string; codes: string[] }) => memberApi.assignRoles(id, codes),
    onSuccess: async (next) => {
      setEditing(null);
      await Promise.all([
        qc.invalidateQueries({ queryKey: ['flow', 'members'] }),
        qc.invalidateQueries({ queryKey: ['flow', 'me'] }),
      ]);
      message.success(`已更新「${next.name}」的角色：${(next.roles ?? []).map((r) => r.name).join('、')}`);
    },
    onError: (e: Error) => message.error(e.message),
  });

  const openEditor = (member: MemberBrief) => {
    setEditing(member);
    setPicked((member.roles ?? []).map((r) => r.code));
  };

  const columns: TableColumnsType<MemberBrief> = [
    {
      title: '成员',
      dataIndex: 'name',
      key: 'name',
      render: (_, record) => (
        <Space direction="vertical" size={0}>
          <Text strong>{record.name || `账号 ${record.accountId}`}</Text>
          <Text type="secondary" style={{ fontSize: 12 }}>
            账号 {record.accountId}
            {record.title ? ` · ${record.title}` : ''}
          </Text>
        </Space>
      ),
    },
    {
      title: '当前角色',
      key: 'currentRole',
      width: 140,
      render: (_, record) =>
        record.currentRole ? <Tag color="blue">{record.currentRole.name}</Tag> : <Text type="secondary">未设定</Text>,
    },
    {
      title: '可担任角色',
      key: 'roles',
      render: (_, record) => (
        <Space size={4} wrap>
          {(record.roles ?? []).map((r) => (
            <Tag key={r.code} color={r.code === 'boss' ? 'red' : 'default'}>
              {r.name}
            </Tag>
          ))}
          {(record.roles ?? []).length === 0 ? <Text type="secondary">暂无</Text> : null}
        </Space>
      ),
    },
    {
      title: '操作',
      key: 'action',
      width: 120,
      render: (_, record) =>
        isBoss ? (
          <Button type="link" size="small" onClick={() => openEditor(record)}>
            改角色
          </Button>
        ) : (
          <Text type="secondary" style={{ fontSize: 12 }}>只读</Text>
        ),
    },
  ];

  return (
    <Space direction="vertical" size={16} style={{ display: 'flex' }}>
      <Card>
        <Space direction="vertical" size={6}>
          <Text strong style={{ fontSize: 16 }}>成员与角色</Text>
          <Paragraph type="secondary" style={{ margin: 0, fontSize: 12 }}>
            工作角色决定"能看到哪些任务队列"，与 IAM 账号权限是两套东西：账号管登录与接口权限，角色管工作台里你接哪一段活。
            老板可以给自己加角色，这样一个人也能把整条链路跑完。
          </Paragraph>
        </Space>
      </Card>

      {data && !isBoss ? (
        <Alert
          type="info"
          showIcon
          message="只有当前角色是老板（boss）时才能修改成员角色"
          description={
            <Text type="secondary">
              你当前是「{data.currentRoleCode}」，可以在这里查看每个人的角色分工；需要调整请把工作角色切到老板。
            </Text>
          }
        />
      ) : null}

      <Card title="全部成员" extra={<Text type="secondary" style={{ fontSize: 12 }}>共 {data?.members?.length ?? 0} 人</Text>}>
        {data?.members?.length ? (
          <Table<MemberBrief>
            rowKey="id"
            columns={columns}
            dataSource={data.members}
            pagination={false}
            loading={membersQuery.isLoading}
          />
        ) : (
          <Empty description={membersQuery.isLoading ? '加载中' : '还没有成员（成员在首次登录工作台时自动建档）'} />
        )}
      </Card>

      <Modal
        open={!!editing}
        title={editing ? `设定「${editing.name}」可担任的角色` : '设定角色'}
        okText="保存"
        cancelText="取消"
        confirmLoading={saveMutation.isPending}
        onCancel={() => setEditing(null)}
        onOk={() => {
          if (!editing) return;
          if (picked.length === 0) {
            message.warning('至少要保留一个角色');
            return;
          }
          saveMutation.mutate({ id: editing.id, codes: picked });
        }}
      >
        <Space direction="vertical" size={12} style={{ display: 'flex' }}>
          <Text type="secondary" style={{ fontSize: 12 }}>
            勾选即授予。取消勾选会收回该角色，但已经落在该角色名下的历史任务不会被删掉。
          </Text>
          <Checkbox.Group
            value={picked}
            onChange={(values) => setPicked(values as string[])}
            style={{ display: 'block' }}
          >
            <Space direction="vertical" size={8}>
              {allRoles.map((role) => (
                <Checkbox key={role.code} value={role.code}>
                  <Space size={6}>
                    <Text>{role.name}</Text>
                    <Text type="secondary" style={{ fontSize: 12 }}>
                      {role.code} · {role.stage}
                    </Text>
                    {editing?.currentRoleId === role.id ? <Tag color="blue">当前角色</Tag> : null}
                  </Space>
                </Checkbox>
              ))}
            </Space>
          </Checkbox.Group>
        </Space>
      </Modal>
    </Space>
  );
}
