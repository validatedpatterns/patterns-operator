import * as React from 'react';
import { fetchVaultJobStatus, VaultJobStatus } from '../api';

const POLL_INTERVAL_MS = 5000;

/**
 * Polls the latest vault injection Job for a pattern until it completes or fails.
 */
export function useVaultJobPolling(patternName: string) {
  const [vaultJobStatus, setVaultJobStatus] = React.useState<VaultJobStatus | null>(null);
  const [checkingVaultStatus, setCheckingVaultStatus] = React.useState(false);

  const checkVaultJobStatus = React.useCallback(async () => {
    if (!patternName) {
      return;
    }

    try {
      setCheckingVaultStatus(true);
      const status = await fetchVaultJobStatus(patternName);
      setVaultJobStatus(status);

      if (status.status === 'running' || status.status === 'pending') {
        setTimeout(() => {
          checkVaultJobStatus();
        }, POLL_INTERVAL_MS);
      }
    } catch (err) {
      console.error('Error checking vault job status:', err);
    } finally {
      setCheckingVaultStatus(false);
    }
  }, [patternName]);

  return { vaultJobStatus, setVaultJobStatus, checkingVaultStatus, checkVaultJobStatus };
}
