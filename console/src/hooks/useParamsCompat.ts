import * as ReactRouterDom from 'react-router-dom';

const isV6 = typeof (ReactRouterDom as any).useNavigate === 'function';

// React Router v5 (OCP < 4.22): useParams may not work if the console
// framework doesn't expose route params through the standard context.
// useRouteMatch explicitly matches the current URL against the given pattern.
// React Router v6 (OCP 4.22+): useParams is the standard API.
export const useParamsCompat: (pattern: string) => Record<string, string> = isV6
  ? () => (ReactRouterDom as any).useParams()
  : (pattern: string) => {
      const match = (ReactRouterDom as any).useRouteMatch(pattern);
      return match?.params || {};
    };
