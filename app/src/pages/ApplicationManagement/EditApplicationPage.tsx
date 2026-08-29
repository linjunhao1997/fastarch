import React, { useState, useEffect } from "react";
import { Form, Input, Button, message } from "antd";
import { useNavigate, useParams } from "react-router";
import { applicationApi } from "../../api/iam";
import { useApiErrorNotification } from "../../hooks/useApiError";

export default function EditApplicationPage() {
  const navigate = useNavigate();
  const { id } = useParams<{ id: string }>();
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const showNotification = useApiErrorNotification();

  useEffect(() => {
    const fetchApplication = async () => {
      if (!id) return;
      setLoading(true);
      try {
        const response = await applicationApi.get(parseInt(id));
        form.setFieldsValue({
          name: response.name,
          description: response.description,
        });
      } catch (error) {
        showNotification(error, "获取应用信息失败");
      } finally {
        setLoading(false);
      }
    };
    fetchApplication();
  }, [id, form, showNotification]);

  const onFinish = async (values: { name: string; description: string }) => {
    if (!id) return;
    setLoading(true);
    try {
      await applicationApi.update(parseInt(id), values);
      message.success("应用更新成功");
      navigate('/application_management/application_list');
    } catch (error) {
      showNotification(error, "应用更新失败");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div>
      <h2>编辑应用</h2>
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
            保存
          </Button>
          <Button onClick={() => navigate('/application_management/application_list')}>
            取消
          </Button>
        </Form.Item>
      </Form>
    </div>
  );
}