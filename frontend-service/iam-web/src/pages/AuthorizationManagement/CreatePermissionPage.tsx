import { useEffect, useState } from "react";
import { Form, Input, Button, Radio, Select, message } from "antd";
import { useNavigate } from "react-router";
import { applicationApi, permissionApi } from "../../api/iam";
import { useApplicationContext } from "../../context/ApplicationContext";
import { useApiErrorNotification } from "../../hooks/useApiError";
import { useSubmitLock } from "../../hooks/useSubmitLock";
import { ACTIONS, PERMISSION_KIND_LABELS, type PermissionKind } from "../../types";

const KIND_OPTIONS = [
  { label: PERMISSION_KIND_LABELS.menu, value: 'menu' as PermissionKind },
  { label: PERMISSION_KIND_LABELS.http_api, value: 'http_api' as PermissionKind },
  { label: PERMISSION_KIND_LABELS.grpc, value: 'grpc' as PermissionKind },
];

export default function CreatePermissionPage() {
  const navigate = useNavigate();
  const [form] = Form.useForm();
  const { isSubmitting, run } = useSubmitLock();
  const showNotification = useApiErrorNotification();
  const [applications, setApplications] = useState<{ label: string; value: string }[]>([]);
  // 默认带入当前选中应用（specs/008 US4），保持可改；资源树选择器随 useWatch 联动
  const { currentApplicationId } = useApplicationContext();

  useEffect(() => {
    if (currentApplicationId) {
      form.setFieldsValue({ applicationId: currentApplicationId });
    }
  }, [currentApplicationId, form]);

  const loadApplications = () => {
    applicationApi.list(1, 1000).then((response) => {
      setApplications(response.applications.map((app) => ({ label: app.name, value: app.id })));
    });
  };

  // 资源模型已废弃（重构 #4）：绑定资源选择器移除，resourceIdList 不再提交
  const onFinish = async (values: { applicationId: string; name: string; code: string; kind: PermissionKind; value?: string; description?: string }) => {
    run(async () => {
      await permissionApi.create({
        applicationId: values.applicationId,
        name: values.name,
        code: values.code,
        kind: values.kind,
        value: values.value,
        description: values.description,
      });
      message.success(`${ACTIONS.create}成功`);
      navigate('/authorization_management/permission_list');
    }).catch((error) => showNotification(error, "权限创建失败"));
  };

  return (
    <div>
      <h2>创建权限</h2>
      <Form form={form} layout="vertical" onFinish={onFinish}>
        <Form.Item label="所属应用" name="applicationId" rules={[{ required: true, message: "请选择应用" }]}>
          <Select options={applications} onFocus={loadApplications} placeholder="选择应用" />
        </Form.Item>
        <Form.Item label="权限名称" name="name" rules={[{ required: true, message: "请输入权限名称" }]}>
          <Input />
        </Form.Item>
        <Form.Item label="权限编码" name="code" rules={[{ required: true, message: "请输入权限编码" }]}>
          <Input />
        </Form.Item>
        <Form.Item label="权限类型" name="kind" rules={[{ required: true, message: "请选择权限类型" }]}>
          <Radio.Group options={KIND_OPTIONS} optionType="button" buttonStyle="solid" />
        </Form.Item>
        <Form.Item label="权限值" name="value">
          <Input />
        </Form.Item>
        <Form.Item label="描述" name="description">
          <Input.TextArea />
        </Form.Item>
        <Form.Item>
          <Button type="primary" htmlType="submit" loading={isSubmitting}>{ACTIONS.create}</Button>
          <Button style={{ marginLeft: 8 }} onClick={() => navigate('/authorization_management/permission_list')}>{ACTIONS.cancel}</Button>
        </Form.Item>
      </Form>
    </div>
  );
}
