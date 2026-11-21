import { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  Alert,
  App,
  Button,
  Card,
  Col,
  Descriptions,
  Drawer,
  Empty,
  Form,
  Input,
  Modal,
  Row,
  Segmented,
  Select,
  Space,
  Statistic,
  Table,
  Tag,
  Typography,
} from 'antd';
import type { TableColumnsType } from 'antd';
import { Link } from 'react-router-dom';
import { memberApi, taskApi } from '@/flow/api/flow';
import type { CreateTaskRequest, Task } from '@/flow/types/flow';

const { Text, Paragraph } = Typography;

const STATE_META: Record<string, { color: string; label: string }> = {
  ready: { color: 'green', label: '可开工' },
  blocked: { color: 'orange', label: '被阻塞' },
  done: { color: 'default', label: '已完成' },
  discarded: { color: 'red', label: '已作废' },
};

const STATE_TABS = [
  { label: '全部', value: '' },
  { label: '可开工', value: 'ready' },
  { label: '被阻塞', value: 'blocked' },
  { label: '已完成', value: 'done' },
  { label: '已作废', value: 'discarded' },
];

/**
 * 任务工作台：以「当前工作角色」为视角看任务。
 * 关键一点：任务能不能开工由服务端派生（open 问题数），前端不做任何状态推断。
 */
export default function TaskWorkbenchPage() {
  const qc = useQueryClient();
  // 走 App.useApp() 的实例方法：静态 Modal.confirm / message 在本工程的挂载环境下不渲染（点了像没反应）
  const { message, modal } = App.useApp();
  const [state, setState] = useState('');
  const [scope, setScope] = useState<'mine' | 'created' | 'all'>('mine');
  const [keyword, setKeyword] = useState('');
  const [page, setPage] = useState(1);
  const [createOpen, setCreateOpen] = useState(false);
  const [detail, setDetail] = useState<Task | null>(null);
  const [form] = Form.useForm<CreateTaskRequest>();

  const meQuery = useQuery({ queryKey: ['flow', 'me'], queryFn: memberApi.me });
  const me = meQuery.data;

  const tasksQuery = useQuery({
    queryKey: ['flow', 'tasks', me?.currentRoleId, scope, state, keyword, page],
    queryFn: () => taskApi.list({ currentPage: page, pageSize: 10, keyword, state, scope }),
    enabled: !!me,
  });
  const data = tasksQuery.data;

  const metricsQuery = useQuery({
    queryKey: ['flow', 'metrics', me?.currentRoleId],
    queryFn: () => taskApi.metrics(),
    enabled: !!me?.currentRoleId,
  });
  const metrics = metricsQuery.data;
  const canAll = me?.currentRole?.code === 'boss' || me?.currentRole?.code === 'pjm';
  const scopeTabs = canAll
    ? [
        { label: '我的队列', value: 'mine' },
        { label: '我发起的', value: 'created' },
        { label: '全链路', value: 'all' },
      ]
    : [
        { label: '我的队列', value: 'mine' },
        { label: '我发起的', value: 'created' },
      ];
  const groupedTasks = useMemo(() => {
    const groups = new Map<string, Task[]>();
    for (const task of data?.tasks ?? []) {
      const key = task.roleName || '未分配角色';
      if (!groups.has(key)) groups.set(key, []);
      groups.get(key)!.push(task);
    }
    return Array.from(groups.entries());
  }, [data]);

  const invalidate = async () => {
    await Promise.all([
      qc.invalidateQueries({ queryKey: ['flow', 'tasks'] }),
      qc.invalidateQueries({ queryKey: ['flow', 'me'] }),
      qc.invalidateQueries({ queryKey: ['flow', 'questions'] }),
      qc.invalidateQueries({ queryKey: ['flow', 'metrics'] }),
    ]);
  };

  const switchMutation = useMutation({
    mutationFn: (code: string) => memberApi.switchRole(code),
    onSuccess: async (next) => {
      setPage(1);
      setScope('mine');
      await invalidate();
      message.success(`已切换为「${next.currentRole?.name ?? ''}」`);
    },
    onError: (e: Error) => message.error(e.message),
  });

  const createMutation = useMutation({
    mutationFn: (values: CreateTaskRequest) => taskApi.create(values),
    onSuccess: async () => {
      setCreateOpen(false);
      form.resetFields();
      await invalidate();
      message.success('任务已创建，落在当前角色上');
    },
    onError: (e: Error) => message.error(e.message),
  });

  const closeMutation = useMutation({
    mutationFn: (id: string) => taskApi.close(id),
    onSuccess: async () => {
      await invalidate();
      message.success('任务已完成');
    },
    onError: (e: Error) => message.error(e.message),
  });

  const discardMutation = useMutation({
    mutationFn: (id: string) => taskApi.discard(id),
    onSuccess: async () => {
      await invalidate();
      message.success('任务已作废，挂在上面的问题一并丢弃');
    },
    onError: (e: Error) => message.error(e.message),
  });

  const columns: TableColumnsType<Task> = useMemo(
    () => [
      {
        title: '任务',
        dataIndex: 'title',
        key: 'title',
        render: (value: string, record) => (
          <Space direction="vertical" size={0}>
            <Text strong>{value}</Text>
          {record.brief ? (
              <Paragraph ellipsis={{ rows: 1, tooltip: record.brief }} style={{ margin: 0, maxWidth: 420, color: '#888' }}>
                {record.brief}
              </Paragraph>
            ) : null}
          </Space>
        ),
      },
      {
        title: '链路',
        key: 'chain',
        width: 210,
        render: (_, record) => (
          <Text style={{ fontSize: 12 }}>
            {record.createdRoleName || '—'} → <Text strong>{record.roleName || '—'}</Text>
          </Text>
        ),
      },
      {
        title: '状态',
        dataIndex: 'state',
        key: 'state',
        width: 140,
        render: (value: string, record) => (
          <Space direction="vertical" size={2}>
            <Tag color={STATE_META[value]?.color ?? 'default'}>{STATE_META[value]?.label ?? value}</Tag>
            {record.state === 'blocked' ? (
              <Text type="secondary" style={{ fontSize: 12, whiteSpace: 'nowrap' }}>
                {record.openQuestions} 个问题未闭环
              </Text>
            ) : null}
          </Space>
        ),
      },
      {
        title: '卡在哪',
        key: 'blockers',
        width: 260,
        render: (_, record) =>
          record.blockers.length === 0 ? (
            <Text type="secondary">—</Text>
          ) : (
            <Space direction="vertical" size={2}>
              {record.blockers.slice(0, 2).map((b) => (
                <Text key={b.questionId} style={{ fontSize: 12 }}>
                  等 {b.toRoleName}：{b.body}
                </Text>
              ))}
              {record.blockers.length > 2 ? (
                <Text type="secondary" style={{ fontSize: 12 }}>还有 {record.blockers.length - 2} 条</Text>
              ) : null}
            </Space>
          ),
      },
      {
        title: '更新时间',
        dataIndex: 'updateTime',
        key: 'updateTime',
        width: 170,
        render: (value: string) => (value ? new Date(value).toLocaleString() : '-'),
      },
      {
        title: '操作',
        key: 'action',
        width: 190,
        render: (_, record) => (
          <Space size={0}>
            <Button type="link" size="small" onClick={() => setDetail(record)}>详情</Button>
            {scope === 'mine' && record.state === 'blocked' ? (
              <Link to={`/flow/questions?task=${record.id}`}>
                <Button type="link" size="small">去闭环</Button>
              </Link>
            ) : null}
            {scope === 'mine' && record.state === 'ready' ? (
              <Button
                type="link"
                size="small"
                onClick={() =>
                  modal.confirm({
                    title: '确认完成这个任务？',
                    content: record.title,
                    onOk: async () => {
                      try {
                        await closeMutation.mutateAsync(record.id);
                      } catch {
                        // 错误提示由 mutation 的 onError 统一弹出，吞掉异常避免确认框卡住不关
                      }
                    },
                  })
                }
              >
                完成
              </Button>
            ) : null}
            {scope === 'mine' && record.status === 'open' ? (
              <Button
                type="link"
                size="small"
                danger
                onClick={() =>
                  modal.confirm({
                    title: '作废任务？',
                    content: '作废后，挂在该任务上的未闭环问题会一并丢弃。',
                    okText: '作废',
                    okButtonProps: { danger: true },
                    onOk: async () => {
                      try {
                        await discardMutation.mutateAsync(record.id);
                      } catch {
                        // 错误提示由 mutation 的 onError 统一弹出，吞掉异常避免确认框卡住不关
                      }
                    },
                  })
                }
              >
                作废
              </Button>
            ) : null}
          </Space>
        ),
      },
    ],
    [closeMutation, discardMutation, modal, scope],
  );

  return (
    <Space direction="vertical" size={16} style={{ display: 'flex' }}>
      <Card>
        <Row gutter={16} align="middle">
          <Col flex="auto">
            <Space direction="vertical" size={4}>
              <Space align="center" wrap>
                <Text strong style={{ fontSize: 16 }}>当前工作角色</Text>
                <Select
                  style={{ width: 180 }}
                  value={me?.currentRole?.code}
                  loading={meQuery.isLoading || switchMutation.isPending}
                  onChange={(value) => switchMutation.mutate(value)}
                  options={(me?.roles ?? []).map((r) => ({
                    value: r.code,
                    label: `${r.name}（${r.stage}）`,
                  }))}
                />
                <Tag color="blue">{me?.name || '未登录'}</Tag>
                {me?.currentRole?.code === 'boss' ? (
                  <Link to="/flow/members">
                    <Button size="small">成员与角色</Button>
                  </Link>
                ) : null}
              </Space>
              <Text type="secondary" style={{ fontSize: 12 }}>
                一人可担任多个角色；任务归属于角色，不归属于人。切换角色后看到的是该角色的队列。
              </Text>
            </Space>
          </Col>
          <Col>
            <Space size={32}>
              <Statistic title="可开工" value={data?.readyCount ?? 0} valueStyle={{ color: '#389e0d' }} />
              <Statistic title="被阻塞" value={data?.blockedCount ?? 0} valueStyle={{ color: '#d46b08' }} />
              <Statistic title="待我闭环" value={me?.openQuestions ?? 0} />
            </Space>
          </Col>
        </Row>
        {metrics ? (
          <Row gutter={16} style={{ marginTop: 18 }}>
            <Col span={6}>
              <Statistic title="平均闭环" value={metrics.avgAnswerMinutes.toFixed(1)} suffix="分钟" />
            </Col>
            <Col span={6}>
              <Statistic title="P50 闭环" value={metrics.p50AnswerMinutes.toFixed(1)} suffix="分钟" />
            </Col>
            <Col span={6}>
              <Statistic
                title="重开率"
                value={(metrics.reopenRate * 100).toFixed(1)}
                suffix="%"
                valueStyle={{ color: metrics.reopenRate > 0.2 ? '#cf1322' : undefined }}
              />
            </Col>
            <Col span={6}>
              <Statistic
                title="闭环后停滞"
                value={metrics.stalledTasks}
                suffix="个任务"
                valueStyle={{ color: metrics.stalledTasks > 0 ? '#d46b08' : '#389e0d' }}
              />
            </Col>
          </Row>
        ) : null}
      </Card>

      {scope === 'mine' && data && data.blockedCount > 0 ? (
        <Alert
          type="warning"
          showIcon
          message={`当前角色有 ${data.blockedCount} 个任务被问题卡住`}
          description={
            <Text type="secondary">
              被阻塞的任务先别硬做：去 <Link to="/flow/questions">问题中心</Link> 闭环，或在任务详情里看清卡在谁那里。
            </Text>
          }
        />
      ) : null}

      <Card
        title={
          scope === 'created'
            ? '我发起的任务'
            : scope === 'all'
              ? '全链路任务（按角色）'
              : `${data?.roleName || '当前角色'} 的任务`
        }
        extra={
          <Space>
            <Input.Search
              allowClear
              placeholder="搜索任务标题"
              style={{ width: 220 }}
              onSearch={(value) => {
                setKeyword(value);
                setPage(1);
              }}
            />
            <Button type="primary" onClick={() => setCreateOpen(true)} disabled={!me?.currentRole}>
              新建任务
            </Button>
          </Space>
        }
      >
        <Space direction="vertical" size={12} style={{ display: 'flex' }}>
          <Space wrap>
            <Segmented
              options={scopeTabs}
              value={scope}
              onChange={(value) => {
                setScope(value as 'mine' | 'created' | 'all');
                setPage(1);
              }}
            />
            {scope !== 'mine' ? (
              <Text type="secondary" style={{ fontSize: 12 }}>
                只读视图：任务仍归当前角色所有，完成/作废请切回对应角色。
              </Text>
            ) : null}
          </Space>
          <Segmented
            options={STATE_TABS}
            value={state}
            onChange={(value) => {
              setState(String(value));
              setPage(1);
            }}
          />
          {scope === 'all' ? (
            <Space direction="vertical" size={12} style={{ display: 'flex' }}>
              {groupedTasks.length === 0 ? <Empty description="还没有任务" /> : null}
              {groupedTasks.map(([roleName, roleTasks]) => (
                <Card key={roleName} size="small" title={`${roleName}（${roleTasks.length}）`}>
                  <Table<Task>
                    rowKey="id"
                    columns={columns}
                    dataSource={roleTasks}
                    pagination={false}
                    locale={{ emptyText: <Empty description="该角色暂无任务" /> }}
                  />
                </Card>
              ))}
            </Space>
          ) : (
            <Table<Task>
              rowKey="id"
              loading={tasksQuery.isFetching}
              columns={columns}
              dataSource={data?.tasks ?? []}
              locale={{ emptyText: <Empty description="这个视图暂时没有任务" /> }}
              pagination={{
                current: page,
                pageSize: 10,
                total: Number(data?.page.total ?? 0),
                showTotal: (total) => `共 ${total} 条`,
                onChange: setPage,
              }}
            />
          )}
        </Space>
      </Card>

      <Modal
        title="新建任务"
        open={createOpen}
        onCancel={() => setCreateOpen(false)}
        onOk={async () => {
          const values = await form.validateFields();
          await createMutation.mutateAsync({ ...values, roleCode: me?.currentRole?.code });
        }}
        confirmLoading={createMutation.isPending}
        destroyOnClose
      >
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 16 }}
          message={`任务会落在「${me?.currentRole?.name ?? ''}」这个角色上，谁担任这个角色谁就能看到。`}
        />
        <Form form={form} layout="vertical" preserve={false}>
          <Form.Item name="title" label="任务标题" rules={[{ required: true, message: '请写清要做什么' }]}>
            <Input placeholder="例如：实现任务工作台的前端页面" maxLength={120} />
          </Form.Item>
          <Form.Item name="brief" label="任务说明">
            <Input.TextArea rows={4} placeholder="输入产出要求、验收标准、依赖的上游产物链接…" />
          </Form.Item>
        </Form>
      </Modal>

      <Drawer
        title={detail?.title}
        open={!!detail}
        onClose={() => setDetail(null)}
        width={560}
      >
        {detail ? (
          <Space direction="vertical" size={16} style={{ display: 'flex' }}>
            <Descriptions column={1} size="small" bordered>
              <Descriptions.Item label="状态">
                <Tag color={STATE_META[detail.state]?.color}>{STATE_META[detail.state]?.label ?? detail.state}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label="角色">{detail.roleName}</Descriptions.Item>
              <Descriptions.Item label="更新时间">
                {detail.updateTime ? new Date(detail.updateTime).toLocaleString() : '-'}
              </Descriptions.Item>
            </Descriptions>
            <div>
              <Text strong>任务说明</Text>
              <Paragraph style={{ marginTop: 8, whiteSpace: 'pre-wrap' }}>{detail.brief || '（无）'}</Paragraph>
            </div>
            <div>
              <Text strong>卡住它的问题（{detail.blockers.length}）</Text>
              {detail.blockers.length === 0 ? (
                <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="没有未闭环问题，可以开工" />
              ) : (
                <Space direction="vertical" size={8} style={{ display: 'flex', marginTop: 8 }}>
                  {detail.blockers.map((b) => (
                    <Card key={b.questionId} size="small">
                      <Space direction="vertical" size={4}>
                        <Text>{b.body}</Text>
                        <Space size={8}>
                          <Tag>等 {b.toRoleName}</Tag>
                          <Tag color={b.answerKind === 'evidence' ? 'purple' : 'blue'}>
                            {b.answerKind === 'evidence' ? '需证据' : '选择即可'}
                          </Tag>
                          <Text type="secondary" style={{ fontSize: 12 }}>{b.createdAt}</Text>
                        </Space>
                      </Space>
                    </Card>
                  ))}
                  <Link to={`/flow/questions?task=${detail.id}`}>
                    <Button type="primary">去问题中心闭环</Button>
                  </Link>
                </Space>
              )}
            </div>
          </Space>
        ) : null}
      </Drawer>
    </Space>
  );
}

