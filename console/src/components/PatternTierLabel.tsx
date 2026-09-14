import * as React from 'react';

import { Label, Tooltip } from '@patternfly/react-core';

const TIER_COLORS: Record<string, 'green' | 'blue' | 'orange' | 'grey'> = {
  maintained: 'green',
  tested: 'blue',
  sandbox: 'orange',
};

const TIER_SVG_COLORS: Record<string, { filled: string; outline: string }> = {
  maintained: { filled: '#3e8635', outline: '#3e8635' },
  tested: { filled: '#0066cc', outline: '#0066cc' },
  sandbox: { filled: '#f0ab00', outline: '#f0ab00' },
};

const TIER_DESCRIPTIONS: Record<string, string> = {
  maintained:
    'Rigorously tested through an automated CI pipeline with continuous validation across OpenShift versions. Highest level of validation and prioritized for ongoing maintenance.',
  tested:
    'Undergoes a manual or automated test plan which passes at least once for each new OpenShift Container Platform minor version.',
  sandbox:
    'Entry-level patterns that are deployable onto a freshly installed OpenShift cluster without prior modification. May be work-in-progress.',
};

const TIER_FILLED_BARS: Record<string, number> = {
  maintained: 3,
  tested: 2,
  sandbox: 1,
};

function TierIcon({ tier }: { tier: string }): React.ReactElement | null {
  const colors = TIER_SVG_COLORS[tier];
  if (!colors) return null;
  const filledCount = TIER_FILLED_BARS[tier] ?? 1;
  return (
    <svg
      width="16"
      height="16"
      viewBox="0 0 48 48"
      style={{ verticalAlign: 'middle', marginRight: '4px' }}
    >
      {[0, 1, 2].map((i) => {
        const y = 34 - i * 14;
        const filled = i < filledCount;
        return (
          <rect
            key={i}
            x="4"
            y={y}
            width="40"
            height="10"
            rx="5"
            fill={filled ? colors.filled : 'none'}
            stroke={colors.outline}
            strokeWidth={filled ? 0 : 3}
          />
        );
      })}
    </svg>
  );
}

export function PatternTierLabel({ tier }: { tier: string }): React.ReactElement {
  return (
    <Tooltip content={TIER_DESCRIPTIONS[tier] || tier}>
      <Label
        color={TIER_COLORS[tier] || 'grey'}
        icon={TIER_SVG_COLORS[tier] ? <TierIcon tier={tier} /> : undefined}
      >
        {tier}
      </Label>
    </Tooltip>
  );
}
