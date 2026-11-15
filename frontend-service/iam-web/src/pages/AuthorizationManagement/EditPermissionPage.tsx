import { useEffect, useState } from "react";
import { Form, Input, Button, Select, message, Descriptions, Spin } from "antd";
import { useNavigate, useParams } from "react-router";
import { permissionApi } from "../../api/iam";
import { useApiErrorNotification } from "../../hooks/useApiError";
import { useSubmitLock } from "../../hooks/useSubmitLock";
import { ACTIONS, PERMISSION_KIND_LABELS, type Permission } from "../../types";

export default function EditPermissionPage() {
  const navigate = useNavigate();
  const { id } = useParams<{ id: string }>();
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(true);
  const [permission, setPermission] = useState<Permission | null>(null);
  const showNotification = useApiErrorNotification();
  const { isSubmitting, run } = useSubmitLock();
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
        setPermission(response);
        setVersion(response.version);
      } catch (error) {
        showNotification(error, "获取权限信息失败");
      } finally {
        setLoading(false);
      }
    };
    fetchPermission();
  }, [id, form, showNotification]);

  const kind = permission?.kind;

  // 资源模型已废弃（重构 #4）：绑定资源选择器移除，resourceIdList 不再提交
  const onFinish = async (values: { name: string; enabledStatus: string }) => {
    if (!id) return;
    run(async () => {
      await permissionApi.update(id, {
        name: values.name,
        enabledStatus: values.enabledStatus,
        version,
      });
      message.success(`${ACTIONS.save}成功`);
      navigate('/authorization_management/permission_list');
    }).catch((error) => showNotification(error, "权限保存失败"));
  };

  if (loading) {
    return <Spin style={{ display: 'block', margin: '64px auto' }} />;
  }

  return (
    <div>
      <h2>编辑权限</h2>
      {kind && (
        <Descriptions size="small" column={1} style={{ marginBottom: 16 }}>
          <Descriptions.Item label="权限编码">{permission?.permissionCode}</Descriptions.Item>
          <Descriptions.Item label="权限类型">{PERMISSION_KIND_LABELS[kind]}（创建后不可变更）</Descriptions.Item>
        </Descriptions>
      )}
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
          <Button type="primary" htmlType="submit" loading={isSubmitting} disabled={loading}>{ACTIONS.save}</Button>
          <Button style={{ marginLeft: 8 }} onClick={() => navigate('/authorization_management/permission_list')}>{ACTIONS.cancel}</Button>
        </Form.Item>
      </Form>
    </div>
  );
}
