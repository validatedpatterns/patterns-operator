import * as React from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Card,
  CardBody,
  CardFooter,
  CardHeader,
  CardTitle,
  Flex,
  FlexItem,
  Label,
  LabelGroup,
  Popover,
  Stack,
  StackItem,
  Tooltip,
} from '@patternfly/react-core';
import { Table, Tbody, Td, Th, Thead, Tr } from '@patternfly/react-table';
import {
  ExternalLinkAltIcon,
  InfoCircleIcon,
  OutlinedQuestionCircleIcon,
} from '@patternfly/react-icons';
import { Pattern, ClusterRoleRequirements, NodeRequirement } from '../types';
import { PatternTierLabel } from './PatternTierLabel';
import './PatternCard.css';

type CloudLabelKey = 'aws' | 'gcp' | 'azure';

const CLOUD_LABELS: Record<CloudLabelKey, string> = {
  aws: 'AWS',
  gcp: 'GCP',
  azure: 'Azure',
};

function getCloudProviders(pattern: Pattern): string[] {
  if (!pattern.requirements) return [];
  const providers = new Set<string>();
  for (const role of Object.values(pattern.requirements)) {
    for (const nodeType of [role.compute, role.controlPlane]) {
      if (nodeType) {
        Object.keys(nodeType).forEach((p) => providers.add(p));
      }
    }
  }
  return Array.from(providers);
}

function formatNode(node?: NodeRequirement): string {
  return node && node.replicas !== 0 ? `${node.replicas} × ${node.type}` : '-';
}

function getSizingSummary(role: ClusterRoleRequirements, cloud: string): string | null {
  const control = role.controlPlane?.[cloud];
  const compute = role.compute?.[cloud];
  if (!control && !compute) return null;
  const parts: string[] = [];
  if (control) parts.push(`${control.replicas} control`);
  if (compute) parts.push(`${compute.replicas} compute`);
  return parts.join(' + ');
}

function HubSummary({ pattern, clouds }: { pattern: Pattern; clouds: string[] }) {
  const { t } = useTranslation('plugin__patterns-operator-console-plugin');
  const hub = pattern.requirements?.hub;
  const spoke = pattern.requirements?.spoke;
  const defaultCloud = clouds.includes('aws') ? 'aws' : clouds[0];
  const hubSummary = hub && defaultCloud ? getSizingSummary(hub, defaultCloud) : null;
  const spokeSummary = spoke && defaultCloud ? getSizingSummary(spoke, defaultCloud) : null;

  if (!hubSummary) return null;

  return (
    <div className="pf-v6-u-text-color-subtle">
      <div>
        {spoke ? (
          <>
            {t('Hub')}: {hubSummary}
            <br />
            {t('Spoke')}: {spokeSummary}
          </>
        ) : (
          `${t('Cluster')}: ${hubSummary}`
        )}
      </div>
      {pattern.external_requirements?.cluster_sizing_note && (
        <Tooltip content={pattern.external_requirements.cluster_sizing_note.trim()}>
          <span>
            <InfoCircleIcon /> {t('Additional requirements')}
          </span>
        </Tooltip>
      )}
    </div>
  );
}

function RequirementsPopoverBody({ pattern, clouds }: { pattern: Pattern; clouds: string[] }) {
  const { t } = useTranslation('plugin__patterns-operator-console-plugin');
  const hub = pattern.requirements?.hub;
  const spoke = pattern.requirements?.spoke;

  const roles: { label: string; role: ClusterRoleRequirements }[] = [];
  if (hub) roles.push({ label: spoke ? t('Hub') : t('Cluster'), role: hub });
  if (spoke) roles.push({ label: t('Spoke'), role: spoke });

  return (
    <Stack hasGutter>
      <StackItem>
        {t(
          'This is the sizing that has been tested. The pattern is expected to work on any similarly-sized architecture.',
        )}
      </StackItem>
      {clouds.map((cloud) => (
        <StackItem key={cloud}>
          <Label color="blue" isCompact>
            {CLOUD_LABELS[cloud] || cloud}
          </Label>
          <Table variant="compact" borders={false}>
            <Thead>
              <Tr>
                <Th />
                <Th>{t('Control plane')}</Th>
                <Th>{t('Compute')}</Th>
              </Tr>
            </Thead>
            <Tbody>
              {roles.map(({ label, role }) => (
                <Tr key={label}>
                  <Td noPadding dataLabel="">
                    <b>{label}</b>
                  </Td>
                  <Td noPadding dataLabel={t('Control plane')}>
                    {formatNode(role.controlPlane?.[cloud])}
                  </Td>
                  <Td noPadding dataLabel={t('Compute')}>
                    {formatNode(role.compute?.[cloud])}
                  </Td>
                </Tr>
              ))}
            </Tbody>
          </Table>
        </StackItem>
      ))}
    </Stack>
  );
}

type PatternCardProps = {
  pattern: Pattern;
  isInstalled: boolean;
  isDisabled: boolean;
  navigate: (path: string) => void;
};

export default function PatternCard({
  pattern,
  isInstalled,
  isDisabled,
  navigate,
}: PatternCardProps) {
  const { t } = useTranslation('plugin__patterns-operator-console-plugin');
  const [isVisible, setIsVisible] = React.useState(false);
  const clouds = getCloudProviders(pattern);

  return (
    <Card
      className={`patterns-operator__card ${isDisabled ? 'patterns-operator__card--disabled' : ''}`}
    >
      <CardHeader>
        <Flex
          justifyContent={{ default: 'justifyContentSpaceBetween' }}
          className="patterns-operator__card-header"
        >
          <FlexItem alignSelf={{ default: 'alignSelfCenter' }}>
            <LabelGroup>
              <PatternTierLabel tier={pattern.tier} />
              {isInstalled && <Label color="green">{t('Installed')}</Label>}
            </LabelGroup>
          </FlexItem>
          <FlexItem>
            {pattern.logo ? (
              <img
                src={pattern.logo}
                alt={`${pattern.display_name} logo`}
                className="patterns-operator__pattern-logo"
              />
            ) : null}
          </FlexItem>
        </Flex>
      </CardHeader>
      <CardTitle>
        <Tooltip content={`org: ${pattern.org}`}>
          <span>{pattern.display_name}</span>
        </Tooltip>
      </CardTitle>
      <CardBody>
        <Stack hasGutter>
          {pattern.description && (
            <StackItem>
              <div className="patterns-operator__pattern-description">{pattern.description}</div>
            </StackItem>
          )}
          <StackItem>
            <Flex direction={{ default: 'column' }} rowGap={{ default: 'rowGapSm' }}>
              <FlexItem>
                <Flex spaceItems={{ default: 'spaceItemsNone' }} flexWrap={{ default: 'nowrap' }}>
                  <FlexItem>
                    <b>
                      <span className="pf-v6-u-font-size-sm">Tested requirements</span>
                    </b>
                  </FlexItem>
                  <FlexItem>
                    <Popover
                      aria-label={t('Tested requirements details')}
                      maxWidth="500px"
                      isVisible={isVisible}
                      shouldOpen={(_event, _fn) => setIsVisible(true)}
                      shouldClose={(_event, _fn) => setIsVisible(false)}
                      headerContent={t('Tested requirements')}
                      bodyContent={<RequirementsPopoverBody pattern={pattern} clouds={clouds} />}
                    >
                      <Button
                        variant="plain"
                        isInline
                        aria-label={t('More information about tested requirements')}
                        icon={<OutlinedQuestionCircleIcon />}
                      />
                    </Popover>
                  </FlexItem>
                </Flex>
              </FlexItem>
              {clouds.length > 0 && (
                <FlexItem>
                  <LabelGroup>
                    {clouds.map((cloud) => (
                      <Label key={cloud} color="blue" isCompact>
                        {CLOUD_LABELS[cloud] || cloud}
                      </Label>
                    ))}
                  </LabelGroup>
                </FlexItem>
              )}
              <FlexItem>
                <HubSummary pattern={pattern} clouds={clouds} />
              </FlexItem>
            </Flex>
          </StackItem>
        </Stack>
      </CardBody>
      <CardFooter>
        <Flex direction={{ default: 'column' }}>
          {(pattern.docs_url || pattern.repo_url) && (
            <FlexItem>
              <Flex rowGap={{ default: 'rowGap2xl' }}>
                {pattern.docs_url && (
                  <FlexItem>
                    <Button
                      variant="link"
                      component="a"
                      href={pattern.docs_url}
                      target="_blank"
                      rel="noopener noreferrer"
                      icon={<ExternalLinkAltIcon set="default" />}
                      iconPosition="end"
                    >
                      {t('Docs')}
                    </Button>
                  </FlexItem>
                )}
                {pattern.repo_url && (
                  <FlexItem>
                    <Button
                      variant="link"
                      component="a"
                      href={pattern.repo_url}
                      target="_blank"
                      rel="noopener noreferrer"
                      icon={<ExternalLinkAltIcon set="default" />}
                      iconPosition="end"
                    >
                      {t('Repo')}
                    </Button>
                  </FlexItem>
                )}
              </Flex>
            </FlexItem>
          )}
          <FlexItem>
            <div className="patterns-operator__card-actions">
              {isInstalled ? (
                <Flex columnGap={{ default: 'columnGapSm' }}>
                  <Button
                    variant="secondary"
                    onClick={() =>
                      navigate(`/patterns/secrets/${pattern.catalogKey || pattern.name}`)
                    }
                  >
                    {t('Manage Secrets')}
                  </Button>
                  <Button
                    variant="danger"
                    onClick={() => navigate(`/patterns/uninstall/${pattern.name}`)}
                  >
                    {t('Uninstall')}
                  </Button>
                </Flex>
              ) : (
                <Tooltip
                  content={t(
                    'Only one pattern can be installed at a time. Uninstall the current pattern first.',
                  )}
                  trigger={isDisabled ? 'mouseenter focus' : 'manual'}
                >
                  <Button
                    variant="primary"
                    isDisabled={isDisabled}
                    onClick={() =>
                      navigate(`/patterns/install/${pattern.catalogKey || pattern.name}`)
                    }
                  >
                    {t('Install')}
                  </Button>
                </Tooltip>
              )}
            </div>
          </FlexItem>
        </Flex>
      </CardFooter>
    </Card>
  );
}
