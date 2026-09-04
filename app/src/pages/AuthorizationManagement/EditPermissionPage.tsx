import { useEffect, useState } from "react";
import { Form, Input, Button, Select, message } from "antd";
import { useNavigate, useParams } from "react-router";
import { permissionApi } from "../../api/iam";
import { useApiErrorNotification } from "../../hooks/useApiError";

export default function EditPermissionPage() {
  const navigate = useNavigate();
  const { id } = useParams<{ id: string }>();
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const showNotification = useApiErrorNotification();
  const [version, setVersion] = useState<number>(0);

  useEffect(() => {
    const fetchPermission = async () => {
      if (!id) return;
      try {
        const response = await permissionApi.get(id);
        form.setFieldsValue({
          name: response.name,
          enabledStatus: response.enabledStatus,
        });
        setVersion(response.version);
      } catch (error) {
        showNotification(error, "获取权限信息失败");
      } finally {
        setLoading(false);
      }
    };
    fetchPermission();
  }, [id, form, showNotification]);

  const onFinish = async (values: { name: string; enabledStatus: string }) => {
    if (!id) return;
    setSubmitting(true);
    try {
      await permissionApi.update(id, { ...values, version });
      message.success("权限更新成功");
      navigate('/authorization_management/permission_list');
    } catch (error) {
      showNotification(error, "权限更新失败");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div>
      <h2>编辑权限</h2>
      <Form form={form} layout="vertical" onFinish={onFinish} initialValues={{ enabledStatus: "enabled" }}>
        <Form.Item label="权限名称" name="name" rules={[{ required: true, message: "请输入权限名称" }]}>
          <Input disabled={loading} />
        </Form.Item>
        <Form.Item label="状态" name="enabledStatus" rules={[{ required: true, message: "请选择状态" }]}>
          <Select
            disabled={loading}
            options={[
              { label: "启用", value: "enabled" },
              { label: "禁用", value: "disabled" },
            ]}
          />
        </Form.Item>
        <Form.Item>
          <Button type="primary" htmlType="submit" loading={submitting} disabled={loading}>保存</Button>
          <Button style={{ marginLeft: 8 }} onClick={() => navigate('/authorization_management/permission_list')}>取消</Button>
        </Form.Item>
      </Form>
    </div>
  );
}
