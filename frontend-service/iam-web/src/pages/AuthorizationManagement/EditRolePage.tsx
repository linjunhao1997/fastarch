import { useEffect, useState } from "react";
import { Button, Card, Form, Input, Select, Space, Tree, message } from "antd";
import { useNavigate, useParams } from "react-router";
import { menuApi, permissionApi, roleApi } from "../../api/iam";
import { useApiErrorNotification } from "../../hooks/useApiError";
import { useSubmitLock } from "../../hooks/useSubmitLock";
import { ACTIONS } from "../../types/copywriting";
import type { MenuTreeNode } from "../../types";

/** 菜单树 → antd Tree 数据（key = menu.id） */
interface MenuTreeData {
  key: string;
  title: string;
  children?: MenuTreeData[];
}

const toTreeData = (nodes: MenuTreeNode[]): MenuTreeData[] =>
  nodes.map((node) => ({
    key: node.menu.id,
    title: node.menu.name,
    children: node.children.length > 0 ? toTreeData(node.children) : undefined,
  }));

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

  // 菜单绑定（t_r_role_menu）：全量覆盖语义，独立于基本信息表单保存
  const [menuTreeData, setMenuTreeData] = useState<MenuTreeData[]>([]);
  const [checkedMenuKeys, setCheckedMenuKeys] = useState<string[]>([]);
  const [menuBindingLoading, setMenuBindingLoading] = useState(true);
  const { isSubmitting: isMenuSubmitting, run: runMenu } = useSubmitLock();

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

  useEffect(() => {
    if (!id) return;
    const fetchMenuBinding = async () => {
      try {
        const [tree, bound] = await Promise.all([
          menuApi.listTree(),
          menuApi.listRoleMenus(id),
        ]);
        setMenuTreeData(toTreeData(tree.roots || []));
        setCheckedMenuKeys(bound.menuIds || []);
      } catch (error) {
        showNotification(error, "获取菜单绑定失败");
      } finally {
        setMenuBindingLoading(false);
      }
    };
    fetchMenuBinding();
  }, [id, showNotification]);

  const onSaveMenuBinding = () => {
    if (!id) return;
    runMenu(async () => {
      await menuApi.updateRoleMenus(id, checkedMenuKeys);
      message.success("菜单绑定保存成功");
    }).catch((error) => showNotification(error, "菜单绑定保存失败"));
  };

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

        <Card
          size="small"
          title="绑定菜单"
          style={{ marginBottom: 16 }}
          extra={
            <Space>
              <Button
                size="small"
                type="primary"
                loading={isMenuSubmitting}
                disabled={menuBindingLoading}
                onClick={onSaveMenuBinding}
              >
                保存菜单绑定
              </Button>
            </Space>
          }
        >
          <Tree
            checkable
            selectable={false}
            disabled={menuBindingLoading}
            treeData={menuTreeData}
            checkedKeys={checkedMenuKeys}
            onCheck={(checked) => {
              const keys = Array.isArray(checked) ? checked : checked.checked;
              setCheckedMenuKeys(keys.map(String));
            }}
          />
        </Card>

        <Form.Item>
          <Button type="primary" htmlType="submit" loading={isSubmitting} disabled={loading}>{ACTIONS.save}</Button>
          <Button style={{ marginLeft: 8 }} onClick={() => navigate('/authorization_management/role_management')}>{ACTIONS.cancel}</Button>
        </Form.Item>
      </Form>
    </div>
  );
}
