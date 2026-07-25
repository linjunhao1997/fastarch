import React, { useState } from "react";
import { Form, Input, Button, Select, Card, message } from "antd";
import { useNavigate } from "react-router-dom";
import { accountApi } from "../../api/iam";
import { useApiErrorNotification } from "../../hooks/useApiError";

export default function CreateAccountPage() {
  const [form] = Form.useForm();
  const [submitting, setSubmitting] = useState(false);
  const navigate = useNavigate();
  const showNotification = useApiErrorNotification();

  const onFinish = async (values: { name: string; email: string; kind: string }) => {
    setSubmitting(true);
    try {
      const kindNum = values.kind === "user" ? 0 : 1;
      await accountApi.create({ ...values, kind: kindNum });
      message.success("账号创建成功");
      navigate('/account_management/account_list');
    } catch (error) {
      showNotification(error, "账号创建失败");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div>
      <Card title="创建账号">
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

          <Form.Item
            label="类型"
            name="kind"
            rules={[{ required: true, message: "请选择类型!" }]}
          >
            <Select placeholder="请选择类型">
              <Select.Option value="user">用户</Select.Option>
              <Select.Option value="service">服务</Select.Option>
            </Select>
          </Form.Item>

          <Form.Item>
            <Button type="primary" htmlType="submit" loading={submitting}>
              创建
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