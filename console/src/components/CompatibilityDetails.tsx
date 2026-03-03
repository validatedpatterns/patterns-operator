import * as React from 'react';
import { useTranslation } from 'react-i18next';
import {
  DescriptionList,
  DescriptionListTerm,
  DescriptionListGroup,
  DescriptionListDescription,
  Label,
  List,
  ListItem,
  Stack,
  StackItem,
  Title,
} from '@patternfly/react-core';
import { CompatibilityResult, ClusterInfo, Pattern } from '../types';
import { getCompatibilityColor } from '../compatibility';

interface CompatibilityDetailsProps {
  result: CompatibilityResult;
  clusterInfo: ClusterInfo;
  pattern: Pattern;
}

export function CompatibilityDetails({
  result,
  clusterInfo,
  pattern,
}: CompatibilityDetailsProps): React.ReactElement {
  const { t } = useTranslation('plugin__console-plugin-template');

  const getClusterTypeDisplay = (): string => {
    if (clusterInfo.isBaremetal) {
      return t('Baremetal');
    }
    return clusterInfo.cloudProvider ? clusterInfo.cloudProvider.toUpperCase() : t('Unknown');
  };

  const renderNodeTypes = (types: string[]): React.ReactElement => {
    const uniqueTypes = Array.from(new Set(types.filter(type => type !== 'unknown')));

    if (uniqueTypes.length === 0) {
      return <span>{t('Unknown types')}</span>;
    }

    if (uniqueTypes.length <= 3) {
      return (
        <>
          {uniqueTypes.map((type, index) => (
            <React.Fragment key={type}>
              <code>{type}</code>
              {index < uniqueTypes.length - 1 && ', '}
            </React.Fragment>
          ))}
        </>
      );
    }

    return (
      <>
        <code>{uniqueTypes[0]}</code>
        {uniqueTypes.length > 1 && (
          <span> {t('and {{count}} others', { count: uniqueTypes.length - 1 })}</span>
        )}
      </>
    );
  };

  const renderRequirementsList = (): React.ReactElement => {
    const hubRequirements = pattern.requirements?.hub;
    if (!hubRequirements || (!hubRequirements.compute && !hubRequirements.controlPlane)) {
      return <span>{t('No specific requirements defined')}</span>;
    }

    return (
      <List isPlain>
        {hubRequirements.compute && (
          <ListItem>
            <strong>{t('Compute Nodes')}:</strong>
            <List isPlain component="div">
              {Object.entries(hubRequirements.compute).map(([provider, spec]) => (
                <ListItem key={provider}>
                  {provider.toUpperCase()}: {spec.replicas} × <code>{spec.type}</code>
                </ListItem>
              ))}
            </List>
          </ListItem>
        )}
        {hubRequirements.controlPlane && (
          <ListItem>
            <strong>{t('Control Plane Nodes')}:</strong>
            <List isPlain component="div">
              {Object.entries(hubRequirements.controlPlane).map(([provider, spec]) => (
                <ListItem key={provider}>
                  {provider.toUpperCase()}: {spec.replicas} × <code>{spec.type}</code>
                </ListItem>
              ))}
            </List>
          </ListItem>
        )}
      </List>
    );
  };

  return (
    <Stack hasGutter className="patterns-operator__compatibility-details">
      <StackItem>
        <Title headingLevel="h4" size="md">
          {t('Compatibility Check Details')}
        </Title>
      </StackItem>

      <StackItem>
        <DescriptionList isHorizontal>
          <DescriptionListGroup>
            <DescriptionListTerm>{t('Status')}</DescriptionListTerm>
            <DescriptionListDescription>
              <Label color={getCompatibilityColor(result.status)}>
                {result.status.charAt(0).toUpperCase() + result.status.slice(1)}
              </Label>
            </DescriptionListDescription>
          </DescriptionListGroup>

          <DescriptionListGroup>
            <DescriptionListTerm>{t('Reason')}</DescriptionListTerm>
            <DescriptionListDescription>{result.reason}</DescriptionListDescription>
          </DescriptionListGroup>
        </DescriptionList>
      </StackItem>

      <StackItem>
        <Title headingLevel="h5" size="sm">
          {t('Cluster Information')}
        </Title>
        <DescriptionList isHorizontal>
          <DescriptionListGroup>
            <DescriptionListTerm>{t('Cloud Provider')}</DescriptionListTerm>
            <DescriptionListDescription>
              {getClusterTypeDisplay()}
            </DescriptionListDescription>
          </DescriptionListGroup>

          <DescriptionListGroup>
            <DescriptionListTerm>{t('Worker Nodes')}</DescriptionListTerm>
            <DescriptionListDescription>
              {clusterInfo.totalWorkerNodes} nodes
              {clusterInfo.workerNodes.length > 0 && (
                <div>
                  {t('Types')}: {renderNodeTypes(clusterInfo.workerNodes.map(n => n.instanceType || 'unknown'))}
                </div>
              )}
            </DescriptionListDescription>
          </DescriptionListGroup>

          <DescriptionListGroup>
            <DescriptionListTerm>{t('Control Plane Nodes')}</DescriptionListTerm>
            <DescriptionListDescription>
              {clusterInfo.totalControlPlaneNodes} nodes
              {clusterInfo.controlPlaneNodes.length > 0 && (
                <div>
                  {t('Types')}: {renderNodeTypes(clusterInfo.controlPlaneNodes.map(n => n.instanceType || 'unknown'))}
                </div>
              )}
            </DescriptionListDescription>
          </DescriptionListGroup>
        </DescriptionList>
      </StackItem>

      <StackItem>
        <Title headingLevel="h5" size="sm">
          {t('Pattern Requirements')}
        </Title>
        {renderRequirementsList()}
      </StackItem>

      {result.details && (
        <StackItem>
          <Title headingLevel="h5" size="sm">
            {t('Detailed Analysis')}
          </Title>
          <DescriptionList isHorizontal>
            {result.details.cloudProviderMatch !== undefined && (
              <DescriptionListGroup>
                <DescriptionListTerm>{t('Cloud Provider Support')}</DescriptionListTerm>
                <DescriptionListDescription>
                  <Label color={result.details.cloudProviderMatch ? 'green' : 'red'}>
                    {result.details.cloudProviderMatch ? t('Supported') : t('Not Supported')}
                  </Label>
                </DescriptionListDescription>
              </DescriptionListGroup>
            )}

            {result.details.requiredCompute && (
              <DescriptionListGroup>
                <DescriptionListTerm>{t('Required Compute')}</DescriptionListTerm>
                <DescriptionListDescription>
                  {result.details.requiredCompute.replicas} × <code>{result.details.requiredCompute.type}</code>
                </DescriptionListDescription>
              </DescriptionListGroup>
            )}

            {result.details.actualCompute && (
              <DescriptionListGroup>
                <DescriptionListTerm>{t('Available Compute')}</DescriptionListTerm>
                <DescriptionListDescription>
                  {result.details.actualCompute.count} nodes ({renderNodeTypes(result.details.actualCompute.types)})
                </DescriptionListDescription>
              </DescriptionListGroup>
            )}

            {result.details.requiredControlPlane && (
              <DescriptionListGroup>
                <DescriptionListTerm>{t('Required Control Plane')}</DescriptionListTerm>
                <DescriptionListDescription>
                  {result.details.requiredControlPlane.replicas} × <code>{result.details.requiredControlPlane.type}</code>
                </DescriptionListDescription>
              </DescriptionListGroup>
            )}

            {result.details.actualControlPlane && (
              <DescriptionListGroup>
                <DescriptionListTerm>{t('Available Control Plane')}</DescriptionListTerm>
                <DescriptionListDescription>
                  {result.details.actualControlPlane.count} nodes ({renderNodeTypes(result.details.actualControlPlane.types)})
                </DescriptionListDescription>
              </DescriptionListGroup>
            )}
          </DescriptionList>
        </StackItem>
      )}
    </Stack>
  );
}

export default CompatibilityDetails;