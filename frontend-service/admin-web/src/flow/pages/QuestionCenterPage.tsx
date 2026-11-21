import { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  Alert,
  App,
  Button,
  Card,
  Descriptions,
  Drawer,
  Empty,
  Input,
  Modal,
  Pagination,
  Radio,
  Segmented,
  Select,
  Space,
  Spin,
  Tag,
  Timeline,
  Typography,
} from 'antd';
import { Link, useSearchParams } from 'react-router-dom';
import { eventApi, memberApi, questionApi } from '@/flow/api/flow';
import type { FlowEvent, Question, WorkRole } from '@/flow/types/flow';

const { Text, Paragraph } = Typography;

/** 单条待回答问题的闭环表单：选项 / 说明 / 证据。 */
function OpenQuestionCard({
  question,
  roles,
  currentRoleId,
  onDone,
}: {
  question: Question;
  roles: WorkRole[];
  currentRoleId: string;
  onDone: () => void | Promise<void>;
}) {
  const { message } = App.useApp();
  const recommended = question.options.find((o) => o.rec)?.id ?? question.options[0]?.id;
  const [optionId, setOptionId] = useState<string | undefined>(recommended);
  const [text, setText] = useState('');
  const [evidence, setEvidence] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [escalateOpen, setEscalateOpen] = useState(false);
  const [escalateTo, setEscalateTo] = useState<string>();
  const [escalateBody, setEscalateBody] = useState('');

  const needEvidence = question.answerKind === 'evidence';

  const submit = async () => {
    if (!optionId && !text.trim()) {
      message.warning('请选择一个选项，或写清结论');
      return;
    }
    if (needEvidence && !evidence.trim()) {
      message.error('这类问题必须提交证据（commit / 工单号 / 部署单 / 文档链接）才能闭环');
      return;
    }
    setSubmitting(true);
    try {
      await questionApi.answer({
        questionId: question.id,
        optionId: optionId || undefined,
        text: text.trim() || undefined,
        evidence: evidence.trim() || undefined,
      });
      message.success('问题已闭环，对应任务会自动解冻');
      await onDone();
    } catch (e) {
      message.error((e as Error).message);
    } finally {
      setSubmitting(false);
    }
  };

  const escalate = async () => {
    if (!escalateTo || !escalateBody.trim()) {
      message.warning('请选择升级对象并写清问题');
      return;
    }
    try {
      await questionApi.escalate(question.id, {
        toRoleCode: escalateTo,
        body: escalateBody.trim(),
        options: question.options,
      });
      message.success('已升级出子问题，父问题仍保持阻塞直到子问题闭环');
      setEscalateOpen(false);
      await onDone();
    } catch (e) {
      message.error((e as Error).message);
    }
  };

  return (
    <Card
      size="small"
      title={
        <Space direction="vertical" size={2}>
          <Text>{question.body}</Text>
          <Space size={6} wrap>
            <Tag color="blue">任务：{question.taskTitle || `#${question.taskId}`}</Tag>
            <Tag>
              {question.fromRoleName} → {question.toRoleName}
            </Tag>
            <Tag color={needEvidence ? 'purple' : 'cyan'}>{needEvidence ? '需提交证据' : '选择即可'}</Tag>
            {question.parentQuestionId !== '0' ? <Tag color="gold">升级子问题</Tag> : null}
          </Space>
        </Space>
      }
      extra={
        <Button size="small" onClick={() => setEscalateOpen(true)}>
          答不了，升级
        </Button>
      }
    >
      <Space direction="vertical" size={12} style={{ display: 'flex' }}>
        {question.options.length > 0 ? (
          <Radio.Group
            value={optionId}
            onChange={(e) => setOptionId(e.target.value)}
            style={{ display: 'flex', flexDirection: 'column', gap: 8 }}
          >
            {question.options.map((o) => (
              <Radio key={o.id} value={o.id}>
                <Space size={6}>
                  <Text>{o.label}</Text>
                  {o.rec ? <Tag color="green">AI 推荐</Tag> : null}
                </Space>
              </Radio>
            ))}
          </Radio.Group>
        ) : null}

        <Input.TextArea
          rows={2}
          value={text}
          onChange={(e) => setText(e.target.value)}
          placeholder={question.options.length > 0 ? '补充说明（可选）' : '写下你的结论'}
        />

        {needEvidence ? (
          <Input
            value={evidence}
            onChange={(e) => setEvidence(e.target.value)}
            placeholder="证据必填：commit / 工单号 / 部署单 / 文档链接"
            status={evidence.trim() ? undefined : 'warning'}
          />
        ) : (
          <Input
            value={evidence}
            onChange={(e) => setEvidence(e.target.value)}
            placeholder="证据（可选）：commit / 工单号 / 部署单 / 文档链接"
          />
        )}

        <Space>
          <Button type="primary" loading={submitting} onClick={submit}>
            闭环这个问题
          </Button>
          <Link to={`/flow/tasks`}>
            <Button type="link">回任务工作台</Button>
          </Link>
        </Space>
      </Space>

      <Modal
        title="升级问题"
        open={escalateOpen}
        onCancel={() => setEscalateOpen(false)}
        onOk={escalate}
        okText="升级"
        destroyOnClose
      >
        <Space direction="vertical" size={12} style={{ display: 'flex' }}>
          <Alert
            type="info"
            showIcon
            message="父问题会保持 open，直到子问题闭环后自动回填，最早那个任务的阻塞才会解除。"
          />
          <Select
            placeholder="升级给哪个角色"
            value={escalateTo}
            onChange={setEscalateTo}
            options={roles
              .filter((r) => r.id !== currentRoleId)
              .map((r) => ({ value: r.code, label: `${r.name}（${r.stage}）` }))}
          />
          <Input.TextArea
            rows={3}
            value={escalateBody}
            onChange={(e) => setEscalateBody(e.target.value)}
            placeholder="写清你判断不了的原因，以及需要对方决策的点"
          />
        </Space>
      </Modal>
    </Card>
  );
}

function AnsweredCard({
  question,
  canReopen,
  roles,
  onDone,
  onShowHistory,
}: {
  question: Question;
  canReopen: boolean;
  roles: { id: string; code: string; name: string }[];
  onDone: () => void | Promise<void>;
  onShowHistory: (q: Question) => void;
}) {
  const { message } = App.useApp();
  const [reopenOpen, setReopenOpen] = useState(false);
  const [reason, setReason] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const pickedLabel = question.picked
    ? (question.options.find((o) => o.id === question.picked)?.label ?? question.picked)
    : '';
  const answeredRoleCode = question.answeredRoleId
    ? (roles.find((r) => String(r.id) === String(question.answeredRoleId))?.code ?? '')
    : '';

  const doReopen = async () => {
    if (!reason.trim()) {
      message.warning('请写清为什么打回：哪条答案不可靠、期望补什么');
      return;
    }
    setSubmitting(true);
    try {
      await questionApi.reopen(question.id, reason.trim());
      message.success('已重开，对应任务会重新变成被阻塞；请等新的答案');
      setReopenOpen(false);
      setReason('');
      await onDone();
    } catch (e) {
      message.error((e as Error).message);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Card size="small">
      <Descriptions
        column={1}
        size="small"
        title={<Text>{question.body}</Text>}
        extra={
          <Space>
            {question.status === 'open' ? <Tag color="orange">未闭环</Tag> : <Tag color="green">已闭环</Tag>}
            {question.reopenCount > 0 ? <Tag color="volcano">重开 {question.reopenCount} 次</Tag> : null}
            {question.selfAnswer ? (
            <Tag color={question.fromRoleId !== question.toRoleId ? 'gold' : 'red'}>
              {question.fromRoleId !== question.toRoleId ? '同账号跨角色闭环' : '自问自答'}
            </Tag>
          ) : null}
            {canReopen && question.status === 'answered' ? (
              <Button size="small" danger onClick={() => setReopenOpen(true)}>
                重开
              </Button>
            ) : null}
            <Button size="small" onClick={() => onShowHistory(question)}>留痕</Button>
          </Space>
        }
      >
        <Descriptions.Item label="任务">
          {question.taskTitle || `#${question.taskId}`}（{question.fromRoleName} → {question.toRoleName}）
        </Descriptions.Item>
        <Descriptions.Item label="结论">
          {question.answer || (question.status === 'open' ? '（还没有结论）' : '—')}
        </Descriptions.Item>
        {pickedLabel ? <Descriptions.Item label="所选选项">{pickedLabel}</Descriptions.Item> : null}
        {question.evidence ? <Descriptions.Item label="证据">{question.evidence}</Descriptions.Item> : null}
        {question.status !== 'open' ? (
          <Descriptions.Item label="回答角色">
            {question.answeredRoleName
              ? `${question.answeredRoleName}${answeredRoleCode ? `（${answeredRoleCode}）` : ''}`
              : '—'}
          </Descriptions.Item>
        ) : null}
        {question.status !== 'open' ? (
          <Descriptions.Item label="闭环时间">
            {question.answeredAt ? new Date(question.answeredAt).toLocaleString() : '—'}
          </Descriptions.Item>
        ) : null}
      </Descriptions>

      <Modal
        title="重开这个问题？"
        open={reopenOpen}
        onCancel={() => setReopenOpen(false)}
        onOk={doReopen}
        okText="确认重开"
        okButtonProps={{ danger: true }}
        confirmLoading={submitting}
        destroyOnClose
      >
        <Space direction="vertical" size={12} style={{ display: 'flex' }}>
          <Alert
            type="warning"
            showIcon
            message="重开后问题回到未闭环，对应任务会重新被阻塞；原答案不会消失，会保留在留痕里。"
          />
          <Input.TextArea
            rows={3}
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            maxLength={512}
            showCount
            placeholder="写清哪条答案不可靠、期望补什么证据或结论"
          />
        </Space>
      </Modal>
    </Card>
  );
}

/**
 * 问题中心：员工在 agent / 页面里闭环阻塞问题的入口。
 * 问题闭环只改问题表一行，任务是否解冻由服务端查询期派生，前端不做状态同步。
 */
export default function QuestionCenterPage() {
  const qc = useQueryClient();
  const { message } = App.useApp();
  const [searchParams, setSearchParams] = useSearchParams();
  const taskFilter = searchParams.get('task') ?? '';
  const [scope, setScope] = useState<'mine' | 'raised'>('mine');
  const [status, setStatus] = useState<'open' | 'answered' | 'all'>('open');
  const [page, setPage] = useState(1);
  const [historyFor, setHistoryFor] = useState<Question | null>(null);
  const pageSize = 20;

  const meQuery = useQuery({ queryKey: ['flow', 'me'], queryFn: memberApi.me });
  const me = meQuery.data;

  const listQuery = useQuery({
    queryKey: ['flow', 'questions', scope, status, taskFilter, page, me?.currentRoleId],
    queryFn: () =>
      questionApi.list({
        scope,
        status,
        pageSize,
        currentPage: page,
        // 任务过滤下沉到服务端：否则只看得到当前这一页里的匹配项
        taskId: taskFilter || undefined,
      }),
    enabled: !!me,
  });

  const questions = useMemo(() => listQuery.data?.questions ?? [], [listQuery.data]);

  const historyQuery = useQuery({
    queryKey: ['flow', 'events', 'question', historyFor?.id],
    queryFn: () => eventApi.list({ entity: 'question', entityId: historyFor!.id, limit: 20 }),
    enabled: !!historyFor,
  });

  const refresh = async () => {
    await Promise.all([
      qc.invalidateQueries({ queryKey: ['flow', 'questions'] }),
      qc.invalidateQueries({ queryKey: ['flow', 'tasks'] }),
      qc.invalidateQueries({ queryKey: ['flow', 'me'] }),
    ]);
  };

  const switchMutation = useMutation({
    mutationFn: (code: string) => memberApi.switchRole(code),
    onSuccess: async (next) => {
      await refresh();
      message.success(`已切换为「${next.currentRole?.name ?? ''}」，现在能处理发往这个角色的问题`);
    },
    onError: (e: Error) => message.error(e.message),
  });

  const tabItems = [
    { value: 'open', label: `待我回答${status === 'open' && scope === 'mine' ? `（${listQuery.data?.openCount ?? 0}）` : ''}` },
    { value: 'answered', label: '我答过的' },
    { value: 'raised', label: '我提出的' },
  ];

  return (
    <Space direction="vertical" size={16} style={{ display: 'flex' }}>
      <Card>
        <Space direction="vertical" size={6} style={{ display: 'flex' }}>
          <Space wrap align="center">
            <Text strong>当前角色</Text>
            <Select
              style={{ width: 200 }}
              value={me?.currentRole?.code}
              loading={switchMutation.isPending}
              onChange={(value) => switchMutation.mutate(value)}
              options={(me?.roles ?? []).map((r) => ({ value: r.code, label: `${r.name}（${r.stage}）` }))}
            />
            <Text type="secondary" style={{ fontSize: 12 }}>
              问题的接收方是角色：你切到哪个角色，就能回答发往该角色的问题。
            </Text>
          </Space>
          {taskFilter ? (
            <Space>
              <Tag color="blue">只看任务 #{taskFilter}</Tag>
              <Button
                size="small"
                type="link"
                onClick={() => {
                  const next = new URLSearchParams(searchParams);
                  next.delete('task');
                  setSearchParams(next, { replace: true });
                  setPage(1);
                }}
              >
                清除筛选
              </Button>
            </Space>
          ) : null}
        </Space>
      </Card>

      <Card>
        <Space direction="vertical" size={12} style={{ display: 'flex' }}>
          <Segmented
            options={tabItems}
            value={scope === 'raised' ? 'raised' : status}
            onChange={(value) => {
              const v = String(value);
              setPage(1);
              if (v === 'raised') {
                setScope('raised');
                // 我提出的：未闭环 + 已闭环都要能看到，不能只查 open
                setStatus('all');
                return;
              }
              setScope('mine');
              setStatus(v === 'answered' ? 'answered' : 'open');
            }}
          />

          {listQuery.isLoading ? (
            <Spin />
          ) : questions.length === 0 ? (
            <Empty
              description={
                scope === 'raised'
                  ? '你还没有提出过问题'
                  : status === 'open'
                    ? '没有待你闭环的问题，当前角色可以正常开工'
                    : '还没有已闭环的问题'
              }
            />
          ) : (
            <Space direction="vertical" size={12} style={{ display: 'flex' }}>
              {questions.map((q) =>
                status === 'open' && scope === 'mine' ? (
                  <OpenQuestionCard
                    key={q.id}
                    question={q}
                    roles={me?.roles ?? []}
                    currentRoleId={me?.currentRoleId ?? ''}
                    onDone={refresh}
                  />
                ) : (
                  <AnsweredCard
                    key={q.id}
                    question={q}
                    canReopen={q.status === 'answered' && (q.askedBy === me?.accountId || me?.currentRole?.code === 'boss')}
                    roles={me?.roles ?? []}
                    onDone={refresh}
                    onShowHistory={setHistoryFor}
                  />
                ),
              )}
            </Space>
          )}

          {Number(listQuery.data?.page.total ?? 0) > pageSize ? (
            <Pagination
              align="end"
              current={page}
              pageSize={pageSize}
              total={Number(listQuery.data?.page.total ?? 0)}
              showSizeChanger={false}
              onChange={setPage}
            />
          ) : null}
        </Space>
      </Card>

      <Drawer
        title={historyFor ? `问题留痕：${historyFor.body.slice(0, 24)}…` : '问题留痕'}
        open={!!historyFor}
        onClose={() => setHistoryFor(null)}
        width={520}
      >
        {historyQuery.isLoading ? (
          <Spin />
        ) : (
          <Timeline
            items={(historyQuery.data?.events ?? []).map((e: FlowEvent) => {
              let detail: Record<string, unknown> | null = null;
              if (e.detail) {
                try {
                  detail = JSON.parse(e.detail) as Record<string, unknown>;
                } catch {
                  detail = null;
                }
              }
              const answerRoleName = typeof detail?.answer_role_name === 'string' ? detail.answer_role_name : '';
              const answerRoleCode = typeof detail?.answer_role_code === 'string' ? detail.answer_role_code : '';
              return {
                children: (
                  <Space direction="vertical" size={2}>
                    <Text strong>{e.action}</Text>
                    <Text type="secondary" style={{ fontSize: 12 }}>
                      {e.at} · {e.actorName || `账号 ${e.actor}`}
                    </Text>
                    {answerRoleName ? (
                      <Text style={{ fontSize: 12 }}>
                        回答角色：{answerRoleName}{answerRoleCode ? `（${answerRoleCode}）` : ''}
                      </Text>
                    ) : null}
                    {e.detail ? (
                      <Paragraph style={{ margin: 0, fontSize: 12, color: '#888' }}>{e.detail}</Paragraph>
                    ) : null}
                  </Space>
                ),
              };
            })}
          />
        )}
      </Drawer>
    </Space>
  );
}

