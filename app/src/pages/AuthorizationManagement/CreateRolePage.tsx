import { useState } from "react";
import { Form, Input, Button, message } from "antd";
import { useNavigate } from "react-router";
import { roleApi } from "../../api/iam";
import { useApiErrorNotification } from "../../hooks/useApiError";

export default function CreateRolePage() {
  const navigate = useNavigate();
  const [form] = Form.useForm();
  const [submitting, setSubmitting] = useState(false);
  const showNotification = useApiErrorNotification();

  const onFinish = async (values: { name: string; roleCode: string }) => {
    setSubmitting(true);
    try {
      await roleApi.create(values);
      message.success("角色创建成功");
      navigate('/authorization_management/role_management');
    } catch (error) {
      showNotification(error, "角色创建失败");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div>
      <h2>创建角色</h2>
      <Form form={form} layout="vertical" onFinish={onFinish}>
        <Form.Item label="角色名称" name="name" rules={[{ required: true, message: "请输入角色名称" }]}>
          <Input />
        </Form.Item>
        <Form.Item label="角色编码" name="roleCode" rules={[{ required: true, message: "请输入角色编码" }]}>
          <Input />
        </Form.Item>
        <Form.Item>
          <Button type="primary" htmlType="submit" loading={submitting}>创建</Button>
          <Button style={{ marginLeft: 8 }} onClick={() => navigate('/authorization_management/role_management')}>取消</Button>
        </Form.Item>
      </Form>
    </div>
  );
}
