import { Breadcrumb } from 'antd';
import { Link, useMatches, type UIMatch } from 'react-router-dom';
import { useApplicationContext } from '@/shared/context/ApplicationContext';

type RouteHandle = {
  breadcrumb?: string;
  navigable?: boolean;
};

export default function AutoBreadcrumb() {
  // 面包屑一级 = 当前应用（左上角切换后随之变化，未绑定时不显示）
  const { currentApplication } = useApplicationContext();
  const matches = useMatches() as UIMatch<unknown, RouteHandle>[];
  const routeItems = matches
    .filter((match) => match.handle?.breadcrumb)
    .map((match, index, arr) => {
      const isLast = index === arr.length - 1;
      const canNavigate = match.handle?.navigable !== false && !isLast;

      return {
        title: canNavigate
          ? <Link to={match.pathname}>{match.handle?.breadcrumb}</Link>
          : match.handle?.breadcrumb,
      };
    });

  const items = currentApplication
    ? [{ title: currentApplication.name }, ...routeItems]
    : routeItems;

  return <Breadcrumb items={items} />;
}