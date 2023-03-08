import * as React from 'react';
import { CatalogTileBadge } from '@patternfly/react-catalog-view-extension';
import CheckCircleIcon from '@patternfly/react-icons/dist/esm/icons/check-circle-icon';
import '../main.css';

export default function PatternsCatalogItemBadge() {
  return (
    <CatalogTileBadge
      className="patterns-console-plugin__validated_badge"
      key={0}
      title="Certified"
    >
      <CheckCircleIcon /> Validated
    </CatalogTileBadge>
  );
}
