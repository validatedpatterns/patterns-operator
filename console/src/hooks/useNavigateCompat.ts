import * as React from 'react';
import * as ReactRouterDom from 'react-router-dom';

const hasUseNavigate = typeof (ReactRouterDom as any).useNavigate === 'function';

// React Router v6 removed useHistory in favor of useNavigate.
// OCP 4.22+ ships v6; OCP 4.21 and earlier ship v5.
export const useNavigateCompat: () => (path: string) => void = hasUseNavigate
  ? () => {
      const navigate = (ReactRouterDom as any).useNavigate();
      return React.useCallback((path: string) => navigate(path), [navigate]);
    }
  : () => {
      const history = (ReactRouterDom as any).useHistory();
      return React.useCallback((path: string) => history.push(path), [history]);
    };
