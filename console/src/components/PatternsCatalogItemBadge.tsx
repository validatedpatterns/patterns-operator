import * as React from 'react';
import { CatalogTileBadge } from '@patternfly/react-catalog-view-extension';
import CheckCircleIcon from '@patternfly/react-icons/dist/esm/icons/check-circle-icon';
import Users from '@patternfly/react-icons/dist/esm/icons/users-icon';
import '../main.css';

export default function PatternsCatalogItemBadge(props) {
  if (props.text === 'Validated') {
    return (
      <CatalogTileBadge className="patterns-console-plugin__validated_badge">
        <CheckCircleIcon /> Validated
      </CatalogTileBadge>
    );
  }

  if (props.text === 'Community') {
    return (
      <CatalogTileBadge className="patterns-console-plugin__community_badge">
        <Users /> Community
      </CatalogTileBadge>
    );
  }

  return <CatalogTileBadge>{props.text}</CatalogTileBadge>;
}
