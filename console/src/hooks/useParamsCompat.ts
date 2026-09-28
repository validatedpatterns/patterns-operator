import * as ReactRouterDom from 'react-router-dom';

type RouterCompat = {
  useNavigate: () => unknown;
  useParams: () => Record<string, string>;
  useRouteMatch: (pattern: string) => { params?: Record<string, string> } | null;
};

const router = ReactRouterDom as unknown as RouterCompat;
const hasUseNavigate = typeof router.useNavigate === 'function';

// React Router v5 (OCP < 4.22): useParams may not work if the console
// framework doesn't expose route params through the standard context.
// useRouteMatch explicitly matches the current URL against the given pattern.
// React Router v6 (OCP 4.22+): useParams is the standard API.
export const useParamsCompat: (pattern: string) => Record<string, string> = hasUseNavigate
  ? () => router.useParams()
  : (pattern: string) => {
      const match = router.useRouteMatch(pattern);
      return match?.params || {};
    };
