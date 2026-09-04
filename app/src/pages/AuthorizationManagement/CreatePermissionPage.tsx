import { useEffect, useState } from "react";
import { Form, Input, Button, Select, message } from "antd";
import { useNavigate } from "react-router";
import { applicationApi, permissionApi } from "../../api/iam";
import { useApiErrorNotification } from "../../hooks/useApiError";
import type { Application } from "../../api/iam";

export default function CreatePermissionPage() {
  const navigate = useNavigate();
  const [form] = Form.useForm();
  const [submitting, setSubmitting] = useState(false);
  const [loading, setLoading] = useState(false);
  const [applications, setApplications] = useState<Application[]>([]);
  const showNotification = useApiErrorNotification();

  useEffect(() => {
    const fetchApplications = async () => {
      setLoading(true);
      try {
        const response = await applicationApi.list(1, 100);
        setApplications(response.applications || []);
      } catch (error) {
        showNotification(error, "获取应用列表失败");
      } finally {
        setLoading(false);
      }
    };
    fetchApplications();
  }, [showNotification]);

  const onFinish = async (values: {
    applicationId: number;
    name: string;
    code: string;
    value?: string;
    description?: string;
  }) => {
    setSubmitting(true);
    try {
      await permissionApi.create({
        applicationId: values.applicationId,
        name: values.name,
        code: values.code,
        value: values.value || "",
        description: values.description || "",
      });
      message.success("权限创建成功");
      navigate('/authorization_management/permission_list');
    } catch (error) {
      showNotification(error, "权限创建失败");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div>
      <h2>创建权限</h2>
      <Form form={form} layout="vertical" onFinish={onFinish}>
        <Form.Item label="所属应用" name="applicationId" rules={[{ required: true, message: "请选择所属应用" }]}>
          <Select
            loading={loading}
            options={applications.map((application) => ({
              label: application.name,
              value: Number(application.id),
            }))}
          />
        </Form.Item>
        <Form.Item label="权限名称" name="name" rules={[{ required: true, message: "请输入权限名称" }]}>
          <Input />
        </Form.Item>
        <Form.Item label="权限编码" name="code" rules={[{ required: true, message: "请输入权限编码" }]}>
          <Input />
        </Form.Item>
        <Form.Item label="权限值" name="value">
          <Input />
        </Form.Item>
        <Form.Item label="描述" name="description">
          <Input.TextArea />
        </Form.Item>
        <Form.Item>
          <Button type="primary" htmlType="submit" loading={submitting}>创建</Button>
          <Button style={{ marginLeft: 8 }} onClick={() => navigate('/authorization_management/permission_list')}>取消</Button>
        </Form.Item>
      </Form>
    </div>
  );
}
