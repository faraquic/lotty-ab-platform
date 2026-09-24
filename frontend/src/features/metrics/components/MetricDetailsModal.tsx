import {
  Badge,
  Button,
  Group,
  Modal,
  Select,
  Stack,
  Text,
  Textarea,
  TextInput,
} from '@mantine/core';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { AggregationEditor } from './AggregationEditor';
import { AttributionEditor } from './AttributionEditor';
import {
  aggregationDraftFromMetric,
  attributionDraftFromMetric,
  buildAggregation,
  buildAttribution,
  metricTypeBadgeColor,
  summarizeAggregation,
} from '../lib/aggregation';
import type { AggregationDraft, AttributionDraft } from '../lib/aggregation';
import { METRIC_STATUSES } from '../types';
import type { Metric, MetricStatus, UpdateMetricRequest } from '../types';
import { formatMetricDateTime } from '../lib/format';

interface MetricDetailsModalProps {
  metric: Metric | null;
  isSaving: boolean;
  onClose: () => void;
  onSave: (id: string, request: UpdateMetricRequest) => void;
}

const MAX_KEY_LENGTH = 128;
const MAX_NAME_LENGTH = 256;
const MAX_DESCRIPTION_LENGTH = 4096;

export function MetricDetailsModal({ metric, isSaving, onClose, onSave }: MetricDetailsModalProps) {
  const { t, i18n } = useTranslation();
  const [key, setKey] = useState('');
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [status, setStatus] = useState<MetricStatus | ''>('');
  const [aggregationDraft, setAggregationDraft] = useState<AggregationDraft | null>(null);
  const [attributionDraft, setAttributionDraft] = useState<AttributionDraft | null>(null);
  const [initializedFor, setInitializedFor] = useState<string | null>(null);

  if (metric !== null && initializedFor !== metric.id) {
    setInitializedFor(metric.id);
    setKey(metric.key);
    setName(metric.name);
    setDescription(metric.description ?? '');
    setStatus(metric.status);
    setAggregationDraft(aggregationDraftFromMetric(metric.aggregation));
    setAttributionDraft(attributionDraftFromMetric(metric.attribution));
  }

  const aggregationBuilt =
    metric !== null && aggregationDraft !== null
      ? buildAggregation(metric.metric_type, aggregationDraft)
      : null;
  const attributionBuilt =
    attributionDraft !== null ? buildAttribution(attributionDraft) : null;

  const keyError =
    key.trim().length === 0
      ? t('metrics.keyRequired')
      : key.length > MAX_KEY_LENGTH
        ? t('metrics.keyLength', { min: 3, max: MAX_KEY_LENGTH })
        : null;
  const nameError =
    name.trim().length === 0
      ? t('metrics.nameRequired')
      : name.length > MAX_NAME_LENGTH
        ? t('metrics.nameMaxLength', { count: MAX_NAME_LENGTH })
        : null;
  const descriptionError =
    description.length > MAX_DESCRIPTION_LENGTH
      ? t('metrics.descriptionMaxLength', { count: MAX_DESCRIPTION_LENGTH })
      : null;

  const handleSave = (): void => {
    if (metric === null || isSaving) {
      return;
    }
    if (keyError !== null || nameError !== null || descriptionError !== null) {
      return;
    }
    if (aggregationBuilt?.aggregation === null || attributionBuilt?.attribution === null) {
      return;
    }
    const request: UpdateMetricRequest = {};
    if (!metric.is_builtin && key.trim() !== metric.key) {
      request.key = key.trim();
    }
    if (name.trim() !== metric.name) {
      request.name = name.trim();
    }
    if (description.trim() !== (metric.description ?? '')) {
      request.description = description.trim();
    }
    if (!metric.is_builtin && aggregationBuilt?.aggregation !== undefined) {
      const current = JSON.stringify(metric.aggregation);
      if (JSON.stringify(aggregationBuilt.aggregation) !== current) {
        request.aggregation = aggregationBuilt.aggregation;
      }
    }
    if (!metric.is_builtin && attributionBuilt?.attribution !== undefined) {
      const current = JSON.stringify(metric.attribution);
      if (JSON.stringify(attributionBuilt.attribution) !== current) {
        request.attribution = attributionBuilt.attribution;
      }
    }
    if (status !== '' && status !== metric.status) {
      request.status = status;
    }
    if (Object.keys(request).length === 0) {
      return;
    }
    onSave(metric.id, request);
  };

  const handleClose = (): void => {
    if (!isSaving) {
      setInitializedFor(null);
      onClose();
    }
  };

  return (
    <Modal
      opened={metric !== null}
      onClose={handleClose}
      title={t('metrics.metricDetails')}
      centered
      size="lg"
    >
      {metric === null || aggregationDraft === null || attributionDraft === null ? null : (
        <Stack gap="md">
          {metric.is_builtin ? (
            <Text size="xs" c="dimmed">
              {t('metrics.builtinHint')}
            </Text>
          ) : null}
          <Group gap="xs">
            <Badge size="sm" color={metricTypeBadgeColor(metric.metric_type)} variant="light">
              {t(`metrics.types.${metric.metric_type}`)}
            </Badge>
            <Text size="xs" c="dimmed">
              {t('metrics.typeImmutableHint')}
            </Text>
          </Group>
          <TextInput
            label={t('metrics.key')}
            disabled={isSaving || metric.is_builtin}
            value={key}
            onChange={(event) => {
              setKey(event.currentTarget.value);
            }}
            error={keyError}
          />
          <TextInput
            label={t('metrics.name')}
            disabled={isSaving}
            value={name}
            onChange={(event) => {
              setName(event.currentTarget.value);
            }}
            error={nameError}
          />
          <Textarea
            label={t('metrics.description')}
            description={t('metrics.descriptionClearHint')}
            disabled={isSaving}
            autosize
            minRows={2}
            value={description}
            onChange={(event) => {
              setDescription(event.currentTarget.value);
            }}
            error={descriptionError}
          />
          {metric.is_builtin ? (
            <Stack gap="xs">
              <Text size="sm" fw={500}>
                {t('metrics.aggregation')}
              </Text>
              <Text size="sm" ff="monospace" c="dimmed">
                {summarizeAggregation(metric.metric_type, metric.aggregation)}
              </Text>
            </Stack>
          ) : (
            <AggregationEditor
              metricType={metric.metric_type}
              draft={aggregationDraft}
              error={aggregationBuilt?.error ?? null}
              disabled={isSaving}
              onChange={setAggregationDraft}
            />
          )}
          {metric.is_builtin ? (
            <Text size="sm" c="dimmed">
              {t('metrics.attributionLockedHint')}
            </Text>
          ) : (
            <AttributionEditor
              draft={attributionDraft}
              windowError={attributionBuilt?.error === 'windowDays'}
              fallbackError={attributionBuilt?.error === 'fallback'}
              disabled={isSaving}
              onChange={setAttributionDraft}
            />
          )}
          <Select
            label={t('metrics.status')}
            disabled={isSaving}
            data={METRIC_STATUSES.map((value) => ({
              value,
              label: t(`metrics.statuses.${value}`),
            }))}
            value={status}
            onChange={(next) => {
              setStatus((next ?? '') as MetricStatus | '');
            }}
          />
          <Text size="xs" c="dimmed">
            {t('metrics.metaLine', {
              created: formatMetricDateTime(metric.created_at, i18n.language),
              updated: formatMetricDateTime(metric.updated_at, i18n.language),
            })}
          </Text>
          <Group justify="flex-end" gap="sm">
            <Button variant="subtle" onClick={handleClose} disabled={isSaving}>
              {t('metrics.cancel')}
            </Button>
            <Button onClick={handleSave} loading={isSaving} disabled={isSaving}>
              {t('metrics.save')}
            </Button>
          </Group>
        </Stack>
      )}
    </Modal>
  );
}
