import { useEffect, useState } from 'react';
import { Select } from 'antd';
import { applicationApi } from '../../api/iam';
import ResourceTree from '../../components/ResourceTree';
import { useApplicationContext } from '../../context/ApplicationContext';
import type { Application } from '../../types';

export default function ResourceManagementPage() {
  const [applications, setApplications] = useState<Application[]>([]);
  const [selectedApplicationId, setSelectedApplicationId] = useState<string | undefined>();
  // 默认定位当前选中应用（specs/008 US4），资源新增继承该 applicationId
  const { currentApplicationId } = useApplicationContext();

  useEffect(() => {
    applicationApi.list(1, 1000).then((response) => {
      setApplications(response.applications);
      if (response.applications.length > 0) {
        setSelectedApplicationId((prev) => prev ?? currentApplicationId ?? String(response.applications[0].id));
      }
    });
  }, [currentApplicationId]);

  return (
    <div>
      <h2>资源管理</h2>
      <Select
        style={{ width: 320, marginBottom: 16 }}
        placeholder="选择应用"
        value={selectedApplicationId}
        onChange={(value) => setSelectedApplicationId(value)}
        options={applications.map((app) => ({ label: app.name, value: String(app.id) }))}
      />
      {selectedApplicationId && <ResourceTree applicationId={selectedApplicationId} />}
    </div>
  );
}
