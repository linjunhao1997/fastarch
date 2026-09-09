import { Breadcrumb } from 'antd';
import type { BreadcrumbProps } from 'antd';
import { Link, useMatches, type UIMatch } from 'react-router-dom';

type RouteHandle = { breadcrumb?: string };

export default function AutoBreadcrumb() {
  const matches = useMatches() as UIMatch<unknown, RouteHandle>[];
  const items = matches
    .filter((match) => match.handle?.breadcrumb)
    .map((match) => ({
      title: match.handle?.breadcrumb ?? '',
      href: match.pathname,
    }));

  const itemRender: BreadcrumbProps['itemRender'] = (route, _params, routes) =>
    routes.indexOf(route) === routes.length - 1
      ? route.title
      : <Link to={route.href ?? '/'}>{route.title}</Link>;

  return <Breadcrumb items={items} itemRender={itemRender} />;
}
