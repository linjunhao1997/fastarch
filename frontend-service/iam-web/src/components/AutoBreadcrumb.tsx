import { Breadcrumb } from 'antd';
import { Link, useMatches, type UIMatch } from 'react-router-dom';

type RouteHandle = { 
  breadcrumb?: string; 
  navigable?: boolean;
};

export default function AutoBreadcrumb() {
  const matches = useMatches() as UIMatch<unknown, RouteHandle>[];
  const items = matches
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

  return <Breadcrumb items={items} />;
}