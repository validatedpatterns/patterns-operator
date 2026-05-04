import * as React from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Spinner } from '@patternfly/react-core';
import { PATTERN_OPERATOR_NS, VaultJobStatus } from '../../api';

export type VaultInjectionStatusAlertProps = {
  vaultJobStatus: VaultJobStatus;
  checkingVaultStatus: boolean;
  style?: React.CSSProperties;
};

/**
 * Inline status for the vault secret injection Kubernetes Job (link to console Job page).
 */
export function VaultInjectionStatusAlert({
  vaultJobStatus,
  checkingVaultStatus,
  style,
}: VaultInjectionStatusAlertProps) {
  const { t } = useTranslation('plugin__patterns-operator-console-plugin');

  return (
    <Alert
      style={style}
      variant={
        vaultJobStatus.status === 'succeeded'
          ? 'success'
          : vaultJobStatus.status === 'failed'
          ? 'danger'
          : 'info'
      }
      title={t('Vault Secret Injection')}
      isInline
    >
      <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
        {(vaultJobStatus.status === 'running' ||
          vaultJobStatus.status === 'pending' ||
          checkingVaultStatus) && <Spinner aria-label={t('Checking vault status')} size="md" />}
        <span>{vaultJobStatus.message}</span>
      </div>
      {vaultJobStatus.jobName && (
        <p style={{ marginTop: '8px', fontSize: '0.9em', color: '#4f5255' }}>
          {t('Job')}:{' '}
          <a href={`/k8s/ns/${PATTERN_OPERATOR_NS}/jobs/${vaultJobStatus.jobName}`}>
            <code>{vaultJobStatus.jobName}</code>
          </a>
        </p>
      )}
    </Alert>
  );
}
