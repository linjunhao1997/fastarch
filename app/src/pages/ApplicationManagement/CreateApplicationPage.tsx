import { useState } from "react";
import { Form, Input, Button, message } from "antd";
import { useNavigate } from "react-router";
import { applicationApi } from "../../api/iam";
import { useApiErrorNotification } from "../../hooks/useApiError";

export default function CreateApplicationPage() {
  const navigate = useNavigate();
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const showNotification = useApiErrorNotification();

  const onFinish = async (values: { name: string; description: string }) => {
    setLoading(true);
    try {
      await applicationApi.create(values);
      message.success("应用创建成功");
      navigate('/application_management/application_list');
    } catch (error) {
      showNotification(error, "应用创建失败");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div>
      <h2>创建应用</h2>
      <Form
        form={form}
        layout="vertical"
        onFinish={onFinish}
        initialValues={{
          name: "",
          description: "",
        }}
      >
        <Form.Item
          label="应用名称"
          name="name"
          rules={[{ required: true, message: "请输入应用名称" }]}
        >
          <Input />
        </Form.Item>
        <Form.Item label="描述" name="description">
          <Input.TextArea />
        </Form.Item>
        <Form.Item>
          <Button type="primary" htmlType="submit" loading={loading}>
            创建
          </Button>
          <Button onClick={() => navigate('/application_management/application_list')}>
            取消
          </Button>
        </Form.Item>
      </Form>
    </div>
  );
}
