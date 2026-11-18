import { Breadcrumb } from 'antd';
import { Link, useMatches, type UIMatch } from 'react-router-dom';
import { useConsole } from '@/shared/context/ConsoleContext';

type RouteHandle = {
  breadcrumb?: string;
  navigable?: boolean;
};

export default function AutoBreadcrumb() {
  // 面包屑一级 = 当前控制台（登录即锁定，ADR 0007）
  const { label } = useConsole();
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

  return <Breadcrumb items={[{ title: label }, ...routeItems]} />;
}