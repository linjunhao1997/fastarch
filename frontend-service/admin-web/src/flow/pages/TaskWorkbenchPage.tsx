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
  in_progress: { color: 'blue', label: '进行中' },
  submitted: { color: 'purple', label: '已提交' },
  done: { color: 'default', label: '已完成' },
  discarded: { color: 'red', label: '已作废' },
};

/** 规划类角色建完任务后不是自己开工，而是等下游拆解。 */
const PLANNING_ROLE_CODES = new Set(['boss', 'analyst', 'pm', 'arch']);

function taskStateMeta(task: Task, roleCode?: string) {
  const base = STATE_META[task.state] ?? { color: 'default', label: task.state };
  if (task.state !== 'ready') return base;
  if ((task.childTotal ?? 0) > 0) return { color: 'cyan', label: '进行中' };
  if (roleCode && PLANNING_ROLE_CODES.has(roleCode)) return { color: 'blue', label: '待拆解' };
  return base;
}

type Scope = 'mine' | 'upstream' | 'created' | 'all';

const STATE_TABS = [
  { label: '全部', value: '' },
  { label: '待处理', value: 'ready' },
  { label: '进行中', value: 'in_progress' },
  { label: '已提交', value: 'submitted' },
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
  const [scope, setScope] = useState<Scope>('mine');
  const [keyword, setKeyword] = useState('');
  const [page, setPage] = useState(1);
  const [createOpen, setCreateOpen] = useState(false);
  const [parentTask, setParentTask] = useState<Task | null>(null);
  const [detail, setDetail] = useState<Task | null>(null);
  const [artifactTask, setArtifactTask] = useState<Task | null>(null);
  const [artifactMode, setArtifactMode] = useState<'submit' | 'finish'>('submit');
  const [form] = Form.useForm<CreateTaskRequest>();
  const [artifactForm] = Form.useForm<{ artifactType: string; title?: string; contentMd: string }>();

  const meQuery = useQuery({ queryKey: ['flow', 'me'], queryFn: memberApi.me });
  const me = meQuery.data;
  const roleCodeById = useMemo(
    () => new Map((me?.roles ?? []).map((r) => [r.id, r.code] as const)),
    [me?.roles],
  );
  const canAll = me?.currentRole?.code === 'boss';

  const tasksQuery = useQuery({
    queryKey: ['flow', 'tasks', me?.currentRoleId, scope, state, keyword, page],
    queryFn: () => taskApi.list({ currentPage: page, pageSize: 10, keyword, state, scope }),
    enabled: !!me,
  });
  const data = tasksQuery.data;
  const artifactsQuery = useQuery({
    queryKey: ['flow', 'artifacts', detail?.id],
    queryFn: () => taskApi.artifacts(detail!.id),
    enabled: !!detail?.id,
  });

  const scopeTabs = [
    { label: '我的任务', value: 'mine' },
    ...(me?.currentRole?.code !== 'boss' ? [{ label: '上游任务', value: 'upstream' }] : []),
    { label: '我发起的', value: 'created' },
    ...(canAll ? [{ label: '全部', value: 'all' }] : []),
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
      qc.invalidateQueries({ queryKey: ['flow', 'artifacts'] }),
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
      setParentTask(null);
      form.resetFields();
      await invalidate();
      message.success('任务已建在你当前角色名下');
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

  const startMutation = useMutation({
    mutationFn: (task: Task) => taskApi.start(task.id),
    onSuccess: async (pkg) => {
      await invalidate();
      message.success(`已开工，读取 ${pkg.artifacts.length} 份上游物料`);
    },
    onError: (e: Error) => message.error(e.message),
  });

  const submitMutation = useMutation({
    mutationFn: (values: { id: string; artifactType: string; title?: string; contentMd: string }) =>
      taskApi.submit(values.id, {
        artifactType: values.artifactType,
        title: values.title,
        contentMd: values.contentMd,
      }),
    onSuccess: async () => {
      setArtifactTask(null);
      artifactForm.resetFields();
      await invalidate();
      message.success('物料已提交，等待下游或老板确认');
    },
    onError: (e: Error) => message.error(e.message),
  });

  const finishMutation = useMutation({
    mutationFn: (values: { id: string; artifactType?: string; title?: string; contentMd: string }) =>
      taskApi.finish(values.id, {
        artifactType: values.artifactType,
        title: values.title,
        contentMd: values.contentMd,
      }),
    onSuccess: async () => {
      setArtifactTask(null);
      artifactForm.resetFields();
      await invalidate();
      message.success('实施产物已归档，任务已完成');
    },
    onError: (e: Error) => message.error(e.message),
  });

  const acceptMutation = useMutation({
    mutationFn: (id: string) => taskApi.accept(id),
    onSuccess: async () => {
      await invalidate();
      message.success('物料已确认，任务已完成');
    },
    onError: (e: Error) => message.error(e.message),
  });

  const openArtifactModal = (task: Task, mode: 'submit' | 'finish') => {
    const roleCode = roleCodeById.get(task.roleId);
    const defaultType =
      mode === 'submit'
        ? roleCode === 'analyst'
          ? 'requirement_spec'
          : roleCode === 'pm'
            ? 'feature_design'
            : 'tech_plan'
        : roleCode === 'qa'
          ? 'test_report'
          : roleCode === 'ops'
            ? 'deployment_record'
            : 'code_change';
    setArtifactMode(mode);
    setArtifactTask(task);
    artifactForm.setFieldsValue({ artifactType: defaultType, title: task.title, contentMd: '' });
  };

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
        title: '处理角色',
        dataIndex: 'roleName',
        key: 'roleName',
        width: 160,
        render: (value: string) => <Text strong>{value || '—'}</Text>,
      },
      {
        title: '状态',
        dataIndex: 'state',
        key: 'state',
        width: 140,
        render: (_value: string, record) => {
          const meta = taskStateMeta(record, roleCodeById.get(record.roleId));
          return (
            <Space direction="vertical" size={2}>
              <Tag color={meta.color}>{meta.label}</Tag>
              {record.state === 'blocked' ? (
                <Text type="secondary" style={{ fontSize: 12, whiteSpace: 'nowrap' }}>
                  {record.openQuestions} 个问题未闭环
                </Text>
              ) : null}
            </Space>
          );
        },
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
        width: 250,
        render: (_, record) => {
          const pendingChildren =
            record.childOpen ?? Math.max(0, (record.childTotal ?? 0) - (record.childDone ?? 0));
          const roleCode = roleCodeById.get(record.roleId);
          const isPlanning = roleCode === 'analyst' || roleCode === 'pm' || roleCode === 'arch';
          const isExecution = roleCode === 'dev' || roleCode === 'qa' || roleCode === 'ops';
          return (
          <Space size={0}>
            <Button type="link" size="small" onClick={() => setDetail(record)}>详情</Button>
            {scope === 'mine' && record.state === 'blocked' ? (
              <Link to={`/flow/questions?task=${record.id}`}>
                <Button type="link" size="small">去闭环</Button>
              </Link>
            ) : null}
            {scope === 'mine' && isPlanning && (record.state === 'ready' || record.state === 'in_progress') ? (
              <Button type="link" size="small" onClick={() => openArtifactModal(record, 'submit')}>
                提交物料
              </Button>
            ) : null}
            {scope === 'mine' && isPlanning && record.state === 'submitted' ? (
              <Text type="secondary" style={{ fontSize: 12, whiteSpace: 'nowrap' }}>等待下游确认</Text>
            ) : null}
            {scope === 'mine' && isExecution && record.state === 'ready' ? (
              <Button type="link" size="small" onClick={() => startMutation.mutate(record)}>
                开始任务
              </Button>
            ) : null}
            {scope === 'mine' && isExecution && record.state === 'in_progress' && pendingChildren === 0 ? (
              <Button type="link" size="small" onClick={() => openArtifactModal(record, 'finish')}>
                完成任务
              </Button>
            ) : null}
            {scope === 'mine' && isExecution && record.state === 'in_progress' && pendingChildren > 0 ? (
              <Text type="secondary" style={{ fontSize: 12, whiteSpace: 'nowrap' }}>
                等 {pendingChildren} 个下级完成
              </Text>
            ) : null}
            {scope === 'mine' && roleCode === 'boss' && record.state === 'ready' && pendingChildren === 0 ? (
              <Button
                type="link"
                size="small"
                onClick={() =>
                  modal.confirm({
                    title: '确认完成这个目标？',
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
            {scope === 'mine' && roleCode === 'boss' && record.state === 'ready' && pendingChildren > 0 ? (
              <Text type="secondary" style={{ fontSize: 12, whiteSpace: 'nowrap' }}>
                等 {pendingChildren} 个下级完成
              </Text>
            ) : null}
            {scope !== 'mine' && record.state === 'submitted' && record.roleId !== me?.currentRoleId && pendingChildren === 0 ? (
              <Button type="link" size="small" onClick={() => acceptMutation.mutate(record.id)}>
                确认物料
              </Button>
            ) : null}
            {scope !== 'mine' && record.state === 'submitted' && record.roleId !== me?.currentRoleId && pendingChildren > 0 ? (
              <Text type="secondary" style={{ fontSize: 12, whiteSpace: 'nowrap' }}>
                等 {pendingChildren} 个下级完成
              </Text>
            ) : null}
            {scope !== 'mine' && record.status === 'open' && record.roleId !== me?.currentRoleId && record.state !== 'blocked' ? (
              <Button
                type="link"
                size="small"
                onClick={() => {
                  setParentTask(record);
                  setCreateOpen(true);
                }}
              >
                基于此建我的任务
              </Button>
            ) : null}
            {scope !== 'mine' && record.status === 'open' && record.roleId !== me?.currentRoleId && record.state === 'blocked' ? (
              <Text type="warning" style={{ fontSize: 12, whiteSpace: 'nowrap' }}>
                父任务有问题待闭环
              </Text>
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
          );
        },
      },
    ],
    [
      closeMutation,
      discardMutation,
      modal,
      scope,
      me?.currentRoleId,
      roleCodeById,
      form,
      startMutation,
      acceptMutation,
      openArtifactModal,
    ],
  );

  const detailStateMeta = detail ? taskStateMeta(detail, roleCodeById.get(detail.roleId)) : null;

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
                一人可担任多个角色；任务归人，切换角色后只显示该角色名下的任务。
              </Text>
            </Space>
          </Col>
          <Col>
            <Space size={32}>
              <Statistic title="待处理" value={data?.readyCount ?? 0} valueStyle={{ color: '#389e0d' }} />
              <Statistic title="被阻塞" value={data?.blockedCount ?? 0} valueStyle={{ color: '#d46b08' }} />
              <Statistic title="待我闭环" value={me?.openQuestions ?? 0} />
            </Space>
          </Col>
        </Row>
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
              ? '全部任务树（按角色）'
              : scope === 'upstream'
                ? '上游任务（只读，可基于它建自己的任务）'
                : '我的任务'
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
            <Button
              type="primary"
              onClick={() => {
                setParentTask(null);
                setCreateOpen(true);
              }}
              disabled={!me?.currentRole}
            >
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
                setScope(value as Scope);
                setPage(1);
              }}
            />
            {scope !== 'mine' ? (
              <Text type="secondary" style={{ fontSize: 12 }}>
                只读：任务归创建者所有；要接着做，请基于它新建自己的子任务，不能替别人完成。
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
        title={parentTask ? '基于上游任务新建我的任务' : '新建我的任务'}
        open={createOpen}
        onCancel={() => {
          setCreateOpen(false);
          setParentTask(null);
          form.resetFields();
        }}
        onOk={async () => {
          const values = await form.validateFields();
          await createMutation.mutateAsync({
            ...values,
            parentTaskId: parentTask?.id,
          });
        }}
        confirmLoading={createMutation.isPending}
        destroyOnClose
      >
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 16 }}
          message={
            parentTask
              ? `父任务：${parentTask.title}`
              : `任务归你（${me?.currentRole?.name ?? '当前角色'}）所有，不会改派给别人。`
          }
          description={
            parentTask
              ? '新任务归你和当前角色，父任务不会改派；父任务要等这条及所有下级任务完成后才能完成。'
              : '每级角色只建自己的任务；下游要接着做，请到「上游任务」里基于父任务新建。'
          }
        />
        <Form form={form} layout="vertical" preserve={false}>
          <Form.Item name="title" label="任务标题" rules={[{ required: true, message: '请写清要做什么' }]}>
            <Input placeholder="例如：基于老板目标拆解需求边界" maxLength={120} />
          </Form.Item>
          <Form.Item name="brief" label="任务说明">
            <Input.TextArea rows={4} placeholder="写清你的产出、验收标准，以及你从父任务承接了什么…" />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={artifactMode === 'submit' ? '提交规划物料' : '完成任务并归档产物'}
        open={!!artifactTask}
        width={760}
        okText={artifactMode === 'submit' ? '提交' : '完成'}
        confirmLoading={submitMutation.isPending || finishMutation.isPending}
        onCancel={() => {
          setArtifactTask(null);
          artifactForm.resetFields();
        }}
        onOk={() => artifactForm.submit()}
        destroyOnClose
      >
        <Form
          form={artifactForm}
          layout="vertical"
          preserve={false}
          onFinish={(values) => {
            if (!artifactTask) return;
            if (artifactMode === 'submit') {
              submitMutation.mutate({ id: artifactTask.id, ...values });
            } else {
              finishMutation.mutate({ id: artifactTask.id, ...values });
            }
          }}
        >
          <Form.Item name="artifactType" label="物料类型" rules={[{ required: true, message: '请填物料类型' }]}>
            <Input placeholder="requirement_spec / feature_design / tech_plan / acceptance_criteria" />
          </Form.Item>
          <Form.Item name="title" label="物料标题">
            <Input placeholder="默认使用任务标题" maxLength={120} />
          </Form.Item>
          <Form.Item name="contentMd" label="Markdown 正文" rules={[{ required: true, message: '请填写 Markdown 正文' }]}>
            <Input.TextArea rows={14} placeholder="写清结论、范围、约束、验收标准；这份内容会作为下游的开工物料。" />
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
                <Tag color={detailStateMeta?.color}>{detailStateMeta?.label ?? detail.state}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label="处理角色">{detail.roleName}</Descriptions.Item>
              <Descriptions.Item label="负责人">{detail.ownerName || '—'}</Descriptions.Item>
              <Descriptions.Item label="父任务">
                {detail.parentTaskTitle ? `${detail.parentTaskTitle}（#${detail.parentTaskId}）` : '顶层任务'}
              </Descriptions.Item>
              {detail.childTotal ? (
                <Descriptions.Item label="下级任务">
                  {detail.childDone}/{detail.childTotal} 已完成
                  {detail.childOpen ? `，${detail.childOpen} 个进行中` : ''}；未全部完成前父任务不能完成
                </Descriptions.Item>
              ) : null}
              <Descriptions.Item label="开工物料">
                {detail.requiredArtifactTypes?.length ? detail.requiredArtifactTypes.join('、') : '按角色默认'}
              </Descriptions.Item>
              {detail.startedAt ? (
                <Descriptions.Item label="开工时间">
                  {new Date(detail.startedAt).toLocaleString()}
                </Descriptions.Item>
              ) : null}
              {detail.submittedAt ? (
                <Descriptions.Item label="提交时间">
                  {new Date(detail.submittedAt).toLocaleString()}
                </Descriptions.Item>
              ) : null}
              {detail.contextHash ? (
                <Descriptions.Item label="执行快照">
                  <Text code>{detail.contextHash.slice(0, 16)}…</Text>
                </Descriptions.Item>
              ) : null}
              <Descriptions.Item label="更新时间">
                {detail.updateTime ? new Date(detail.updateTime).toLocaleString() : '-'}
              </Descriptions.Item>
            </Descriptions>
            {detail.roleId !== me?.currentRoleId && detail.status === 'open' && detail.state !== 'blocked' ? (
              <Button
                type="primary"
                onClick={() => {
                  setParentTask(detail);
                  setDetail(null);
                  setCreateOpen(true);
                }}
              >
                基于此任务建我的任务
              </Button>
            ) : null}
            {detail.roleId !== me?.currentRoleId && detail.state === 'blocked' ? (
              <Alert
                type="warning"
                showIcon
                message="这个任务还有未闭环问题，先别往下派生"
                description="等父任务的问题闭环后，再基于它新建你自己的子任务。"
              />
            ) : null}
            <div>
              <Text strong>任务说明</Text>
              <Paragraph style={{ marginTop: 8, whiteSpace: 'pre-wrap' }}>{detail.brief || '（无）'}</Paragraph>
            </div>
            <div>
              <Text strong>Markdown 物料（{artifactsQuery.data?.artifacts.length ?? 0}）</Text>
              {artifactsQuery.isLoading ? (
                <Text type="secondary">加载中…</Text>
              ) : artifactsQuery.data?.artifacts.length ? (
                <Space direction="vertical" size={8} style={{ display: 'flex', marginTop: 8 }}>
                  {artifactsQuery.data.artifacts.map((artifact) => (
                    <Card
                      key={artifact.id}
                      size="small"
                      title={
                        <Space wrap>
                          <Tag color="blue">{artifact.type}</Tag>
                          <Text>{artifact.title || '未命名物料'}</Text>
                          <Text type="secondary">v{artifact.version}</Text>
                          <Tag color={artifact.status === 'accepted' ? 'green' : 'purple'}>{artifact.status}</Tag>
                        </Space>
                      }
                    >
                      <Paragraph
                        style={{ margin: 0, whiteSpace: 'pre-wrap', maxHeight: 220, overflow: 'auto' }}
                      >
                        {artifact.contentMd}
                      </Paragraph>
                    </Card>
                  ))}
                </Space>
              ) : (
                <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="还没有提交物料" />
              )}
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

