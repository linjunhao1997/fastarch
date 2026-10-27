import { useEffect, useState } from "react";
import { Form, Input, Button, Select, message } from "antd";
import { useNavigate } from "react-router";
import { applicationApi, roleApi } from "../../api/iam";
import { useApplicationContext } from "../../context/ApplicationContext";
import { useApiErrorNotification } from "../../hooks/useApiError";
import { useSubmitLock } from "../../hooks/useSubmitLock";
import { ACTIONS } from "../../types/copywriting";

export default function CreateRolePage() {
  const navigate = useNavigate();
  const [form] = Form.useForm();
  const { isSubmitting, run } = useSubmitLock();
  const showNotification = useApiErrorNotification();
  const [applications, setApplications] = useState<{ label: string; value: string }[]>([]);
  // 默认带入当前选中应用（specs/008 US4），保持可改；上下文为空时走必填校验提示
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

  const onFinish = async (values: { applicationId: string; name: string; roleCode: string }) => {
    run(async () => {
      await roleApi.create(values);
      message.success(`${ACTIONS.create}成功`);
      navigate('/authorization_management/role_management');
    }).catch((error) => showNotification(error, "角色创建失败"));
  };

  return (
    <div>
      <h2>创建角色</h2>
      <Form form={form} layout="vertical" onFinish={onFinish}>
        <Form.Item label="所属应用" name="applicationId" rules={[{ required: true, message: "请选择应用" }]}>
          <Select options={applications} onFocus={loadApplications} placeholder="选择应用" />
        </Form.Item>
        <Form.Item label="角色名称" name="name" rules={[{ required: true, message: "请输入角色名称" }]}>
          <Input />
        </Form.Item>
        <Form.Item label="角色编码" name="roleCode" rules={[{ required: true, message: "请输入角色编码" }]}>
          <Input />
        </Form.Item>
        <Form.Item>
          <Button type="primary" htmlType="submit" loading={isSubmitting}>{ACTIONS.create}</Button>
          <Button style={{ marginLeft: 8 }} onClick={() => navigate('/authorization_management/role_management')}>{ACTIONS.cancel}</Button>
        </Form.Item>
      </Form>
    </div>
  );
}
