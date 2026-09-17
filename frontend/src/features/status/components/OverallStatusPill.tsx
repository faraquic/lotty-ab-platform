import { useTranslation } from 'react-i18next';
import { SegmentedSpinner } from '@/components/ui/SegmentedSpinner';
import { useServiceStatus } from '../api/useServiceStatus';
import { aggregateOverallStatus } from '../lib/aggregate';
import {
  ANALYTICS_API_BASE_URL,
  PANEL_API_BASE_URL,
  RUNTIME_API_BASE_URL,
} from '@/shared/config/api';
import type { ServiceProbe } from '../types';

const LABEL_KEYS = {
  operational: 'status.overallShort.operational',
  partial: 'status.overallShort.partial',
  outage: 'status.overallShort.outage',
} as const;

export function OverallStatusPill() {
  const { t } = useTranslation();
  const panel = useServiceStatus('panel', PANEL_API_BASE_URL);
  const runtime = useServiceStatus('runtime', RUNTIME_API_BASE_URL);
  const analytics = useServiceStatus('analytics', ANALYTICS_API_BASE_URL);

  const probes: ServiceProbe[] = [panel.data, runtime.data, analytics.data].filter(
    (probe): probe is ServiceProbe => probe !== undefined,
  );

  if (probes.length === 0) {
    return (
      <span className="status-button-indicator status-button-indicator-loading" aria-hidden="true">
        <SegmentedSpinner size={18} />
      </span>
    );
  }

  const overall = aggregateOverallStatus(probes.map((probe) => probe.status));

  return (
    <span
      className={`status-button-indicator status-button-indicator-${overall}`}
      role="img"
      aria-label={t(LABEL_KEYS[overall])}
    >
      <SegmentedSpinner size={18} />
    </span>
  );
}
