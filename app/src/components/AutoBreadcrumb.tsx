import { Breadcrumb } from "antd";
import { Link, useMatches } from "react-router-dom";

export default function AutoBreadcrumb(props: React.ComponentProps<typeof Breadcrumb>) {
  function itemRender(currentRoute, params, items, paths) {
    const isLast = currentRoute?.path === items[items.length - 1]?.path;

  
    return isLast ? (
      <span>{currentRoute.title}</span>
    ) : (
      <Link to={`/${paths.join("/")}`}>{currentRoute.title}</Link>
    );
  }
  const matches = useMatches();
  const items = matches
    .filter((match) => Boolean(match.handle?.breadcrumb))
    .map((match) => ({
      title:
        typeof match.handle.breadcrumb === "function"
          ? match.handle.breadcrumb(match.data)
          : match.handle.breadcrumb,
      path: match.pathname,
    }));
  return <Breadcrumb itemRender={itemRender} items={items} {...props}/>;
};