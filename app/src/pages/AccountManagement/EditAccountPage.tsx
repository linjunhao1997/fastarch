import { useState, useEffect } from "react";
import { Form, Input, Button, Card, message } from "antd";
import { useNavigate, useParams } from "react-router-dom";
import { accountApi } from "../../api/iam";
import { useApiErrorNotification } from "../../hooks/useApiError";

export default function EditAccountPage() {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const navigate = useNavigate();
  const { id } = useParams<{ id: string }>();
  const showNotification = useApiErrorNotification();
  const [version, setVersion] = useState<number>(0);

  useEffect(() => {
    fetchAccount();
  }, [showNotification]);

  const fetchAccount = async () => {
    try {
      const response = await accountApi.get(id || "");
      form.setFieldsValue({
        name: response.name,
        email: response.email,
      });
      setVersion(response.version);
    } catch (error) {
      showNotification(error, "获取账号信息失败");
    } finally {
      setLoading(false);
    }
  };

  const onFinish = async (values: { name?: string; email?: string }) => {
    setSubmitting(true);
    try {
      await accountApi.update(id || "", { ...values, version });
      message.success("账号更新成功");
      navigate('/account_management/account_list');
    } catch (error) {
      showNotification(error, "账号更新失败");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div>
      <Card title="编辑账号">
        <Form
          form={form}
          layout="vertical"
          onFinish={onFinish}
        >
          <Form.Item
            label="姓名"
            name="name"
            rules={[{ required: true, message: "请输入姓名!" }]}
          >
            <Input placeholder="请输入姓名" />
          </Form.Item>

          <Form.Item
            label="邮箱"
            name="email"
            rules={[
              { required: true, message: "请输入邮箱!" },
              { type: "email", message: "请输入有效的邮箱格式!" }
            ]}
          >
          <Input placeholder="请输入邮箱" />
        </Form.Item>

        <Form.Item label="版本" name="version" hidden initialValue={version}>
          <Input />
        </Form.Item>

          <Form.Item>
            <Button type="primary" htmlType="submit" loading={submitting} disabled={loading}>
              更新
            </Button>
            <Button style={{ marginLeft: 8 }} onClick={() => navigate('/account_management/account_list')}>
              取消
            </Button>
          </Form.Item>
        </Form>
      </Card>
    </div>
  );
}
