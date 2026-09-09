import { useEffect, useState } from "react";
import { Form, Input, Button, Select, message } from "antd";
import { useNavigate, useParams } from "react-router";
import { permissionApi, roleApi } from "../../api/iam";
import { useApiErrorNotification } from "../../hooks/useApiError";
import { useSubmitLock } from "../../hooks/useSubmitLock";
import { ACTIONS } from "../../types/copywriting";

export default function EditRolePage() {
  const navigate = useNavigate();
  const { id } = useParams<{ id: string }>();
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(true);
  const showNotification = useApiErrorNotification();
  const { isSubmitting, run } = useSubmitLock();
  const [version, setVersion] = useState<number>(0);
  const [currentPermissionCodes, setCurrentPermissionCodes] = useState<string[]>([]);
  const [permissionOptions, setPermissionOptions] = useState<{ label: string; value: string }[]>([]);

  useEffect(() => {
    const fetchRole = async () => {
      if (!id) return;
      try {
        const response = await roleApi.get(id);
        form.setFieldsValue({
          name: response.name,
          enabledStatus: response.enabledStatus,
          permissionCodeList: response.permissionCodeList,
        });
        setVersion(response.version);
        setCurrentPermissionCodes(response.permissionCodeList || []);
      } catch (error) {
        showNotification(error, "获取角色信息失败");
      } finally {
        setLoading(false);
      }
    };
    fetchRole();
  }, [id, form, showNotification]);

  useEffect(() => {
    const fetchPermissions = async () => {
      try {
        const response = await permissionApi.list(1, 100);
        setPermissionOptions((response.permissions || []).map(permission => ({
          label: permission.name,
          value: permission.permissionCode,
        })));
      } catch (error) {
        showNotification(error, "获取权限列表失败");
      }
    };
    fetchPermissions();
  }, [showNotification]);

  const onFinish = async (values: { name: string; enabledStatus: string; permissionCodeList?: string[] }) => {
    if (!id) return;
    run(async () => {
      const selected = new Set(values.permissionCodeList || []);
      const previous = new Set(currentPermissionCodes);
      await roleApi.update(id, {
        ...values,
        addedPermissionCodeList: [...selected].filter(code => !previous.has(code)),
        removedPermissionCodeList: [...previous].filter(code => !selected.has(code)),
        version,
      });
      message.success(`${ACTIONS.save}成功`);
      navigate('/authorization_management/role_management');
    }).catch((error) => showNotification(error, "角色保存失败"));
  };

  return (
    <div>
      <h2>编辑角色</h2>
      <Form form={form} layout="vertical" onFinish={onFinish} initialValues={{ enabledStatus: "enabled" }}>
        <Form.Item label="角色名称" name="name" rules={[{ required: true, message: "请输入角色名称" }]}>
          <Input disabled={loading} />
        </Form.Item>

        <Form.Item label="权限" name="permissionCodeList" initialValue={currentPermissionCodes}>
          <Select mode="multiple" options={permissionOptions} disabled={loading} />
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
          <Button style={{ marginLeft: 8 }} onClick={() => navigate('/authorization_management/role_management')}>{ACTIONS.cancel}</Button>
        </Form.Item>
      </Form>
    </div>
  );
}
