import { useCallback, useEffect, useState } from "react";
import { Form, Input, Button, Card, Radio, message } from "antd";
import { useQueryClient } from '@tanstack/react-query';
import { useNavigate, useParams } from "react-router-dom";
import { applicationApi } from "@/iam/api/iam";
import { useApiErrorNotification } from "@/shared/hooks/useApiError";
import { useSubmitLock } from "@/shared/hooks/useSubmitLock";
import { ACTIONS } from "@/shared/types/copywriting";

export default function EditApplicationPage() {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(true);
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { id } = useParams<{ id: string }>();
  const showNotification = useApiErrorNotification();
  const { isSubmitting, run } = useSubmitLock();
  const [version, setVersion] = useState<number>(0);
  const [code, setCode] = useState<string>("");

  const fetchApplication = useCallback(async () => {
    try {
      const response = await applicationApi.get(id || "");
      form.setFieldsValue({
        name: response.name,
        code: response.code,
        appId: response.appId,
        status: response.status || "enabled",
      });
      setCode(response.code);
      setVersion(response.version);
    } catch (error) {
      showNotification(error, "获取应用信息失败");
    } finally {
      setLoading(false);
    }
  }, [form, id, showNotification]);

  useEffect(() => {
    void fetchApplication();
  }, [fetchApplication]);

  const onFinish = async (values: { name?: string; status?: string }) => {
    run(async () => {
      // code 为服务段命名空间，创建后不可变更；仅名称/状态可编辑
      await applicationApi.update(id || "", {
        name: values.name,
        status: values.status,
        version,
      });
      message.success(`${ACTIONS.save}成功`);
      queryClient.invalidateQueries({ queryKey: ['applications'] });
      navigate('/application_management/application_list');
    }).catch((error) => showNotification(error, "应用保存失败"));
  };

  return (
    <div>
      <Card title={`编辑应用${code ? `：${code}` : ""}`}>
        <Form form={form} layout="vertical" onFinish={onFinish}>
          <Form.Item label="服务段编码" name="code" tooltip="权限码命名空间，创建后不可修改">
            <Input disabled />
          </Form.Item>

          <Form.Item label="应用标识（app_id）" name="appId">
            <Input disabled />
          </Form.Item>

          <Form.Item label="应用名称" name="name" rules={[{ required: true, message: "请输入应用名称!" }]}>
            <Input placeholder="展示名称" />
          </Form.Item>

          <Form.Item label="状态" name="status" rules={[{ required: true, message: "请选择状态" }]}>
            <Radio.Group>
              <Radio value="enabled">启用</Radio>
              <Radio value="disabled">禁用</Radio>
            </Radio.Group>
          </Form.Item>

          <Form.Item>
            <Button type="primary" htmlType="submit" loading={isSubmitting} disabled={loading}>
              {ACTIONS.save}
            </Button>
            <Button style={{ marginLeft: 8 }} onClick={() => navigate('/application_management/application_list')}>
              {ACTIONS.cancel}
            </Button>
          </Form.Item>
        </Form>
      </Card>
    </div>
  );
}
