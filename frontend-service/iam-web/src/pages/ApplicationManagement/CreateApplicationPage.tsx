import { useState } from "react";
import { Form, Input, Button, Radio, Card, Modal, Typography, message } from "antd";
import { useQueryClient } from '@tanstack/react-query';
import { useNavigate } from "react-router-dom";
import { applicationApi } from "../../api/iam";
import { useApiErrorNotification } from "../../hooks/useApiError";
import { useSubmitLock } from "../../hooks/useSubmitLock";
import { ACTIONS } from "../../types/copywriting";

export default function CreateApplicationPage() {
  const [form] = Form.useForm();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const showNotification = useApiErrorNotification();
  const { isSubmitting, run } = useSubmitLock();
  // 创建成功后一次性展示的 secret 明文（关闭后不可再查）
  const [createdSecret, setCreatedSecret] = useState<string | null>(null);

  const onFinish = async (values: { name: string; code: string; status: string }) => {
    run(async () => {
      const response = await applicationApi.create(values);
      queryClient.invalidateQueries({ queryKey: ['applications'] });
      if (response.secret) {
        setCreatedSecret(response.secret);
      } else {
        message.success(`${ACTIONS.create}成功`);
        navigate('/application_management/application_list');
      }
    }).catch((error) => showNotification(error, "应用创建失败"));
  };

  return (
    <div>
      <Card title="创建应用">
        <Form
          form={form}
          layout="vertical"
          initialValues={{ status: "enabled" }}
          onFinish={onFinish}
        >
          <Form.Item
            label="应用名称"
            name="name"
            rules={[{ required: true, message: "请输入应用名称!" }]}
          >
            <Input placeholder="展示名称" />
          </Form.Item>

          <Form.Item
            label="服务段编码"
            name="code"
            tooltip="权限码命名空间（如 iam/order/crm）：小写字母开头，仅小写字母、数字、下划线；创建后不可修改"
            rules={[
              { required: true, message: "请输入服务段编码!" },
              { pattern: /^[a-z][a-z0-9_]*$/, message: "小写字母开头，仅小写字母/数字/下划线" },
              { max: 32, message: "最长 32 个字符" },
            ]}
          >
            <Input placeholder="如 order" />
          </Form.Item>

          <Form.Item label="状态" name="status" rules={[{ required: true, message: "请选择状态" }]}>
            <Radio.Group>
              <Radio value="enabled">启用</Radio>
              <Radio value="disabled">禁用</Radio>
            </Radio.Group>
          </Form.Item>

          <Form.Item>
            <Button type="primary" htmlType="submit" loading={isSubmitting}>
              {ACTIONS.create}
            </Button>
            <Button style={{ marginLeft: 8 }} onClick={() => navigate('/application_management/application_list')}>
              {ACTIONS.cancel}
            </Button>
          </Form.Item>
        </Form>
      </Card>

      <Modal
        title="应用创建成功"
        open={createdSecret !== null}
        closable={false}
        maskClosable={false}
        footer={
          <Button
            type="primary"
            onClick={() => {
              setCreatedSecret(null);
              navigate('/application_management/application_list');
            }}
          >
            我已保存，关闭
          </Button>
        }
      >
        <Typography.Paragraph>
          应用 secret 仅此一次展示，关闭后无法再次查看，请立即妥善保存：
        </Typography.Paragraph>
        <Typography.Paragraph copyable code style={{ wordBreak: 'break-all' }}>
          {createdSecret}
        </Typography.Paragraph>
      </Modal>
    </div>
  );
}
