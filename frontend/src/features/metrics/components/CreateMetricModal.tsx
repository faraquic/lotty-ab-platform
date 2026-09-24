import { Button, Group, Modal, Select, Stack, Textarea, TextInput } from '@mantine/core';
import { useForm } from '@mantine/form';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { AggregationEditor } from './AggregationEditor';
import { AttributionEditor } from './AttributionEditor';
import { buildAggregation, buildAttribution, emptyAggregationDraft } from '../lib/aggregation';
import type { AggregationDraft, AttributionDraft } from '../lib/aggregation';
import { METRIC_TYPES, isMetricType } from '../types';
import type { CreateMetricRequest, MetricType } from '../types';

export interface CreateMetricFormValues {
  key: string;
  name: string;
  description: string;
  metric_type: MetricType | '';
}

interface CreateMetricModalProps {
  opened: boolean;
  isPending: boolean;
  onClose: () => void;
  onSubmit: (request: CreateMetricRequest) => void;
}

const MIN_KEY_LENGTH = 3;
const MAX_KEY_LENGTH = 128;
const MAX_NAME_LENGTH = 256;
const MAX_DESCRIPTION_LENGTH = 4096;

export function CreateMetricModal({ opened, isPending, onClose, onSubmit }: CreateMetricModalProps) {
  const { t, i18n } = useTranslation();
  const [aggregationDraft, setAggregationDraft] = useState<AggregationDraft>(emptyAggregationDraft);
  const [attributionDraft, setAttributionDraft] = useState<AttributionDraft>({
    requireExposure: true,
    windowDaysRaw: '7',
    fallback: 'subject',
  });

  const form = useForm<CreateMetricFormValues>({
    initialValues: { key: '', name: '', description: '', metric_type: '' },
    validate: {
      key: (value) => {
        if (value.trim().length === 0) {
          return i18n.t('metrics.keyRequired');
        }
        if (value.trim().length < MIN_KEY_LENGTH || value.length > MAX_KEY_LENGTH) {
          return i18n.t('metrics.keyLength', { min: MIN_KEY_LENGTH, max: MAX_KEY_LENGTH });
        }
        return null;
      },
      name: (value) => {
        if (value.trim().length === 0) {
          return i18n.t('metrics.nameRequired');
        }
        if (value.length > MAX_NAME_LENGTH) {
          return i18n.t('metrics.nameMaxLength', { count: MAX_NAME_LENGTH });
        }
        return null;
      },
      description: (value) =>
        value.length > MAX_DESCRIPTION_LENGTH
          ? i18n.t('metrics.descriptionMaxLength', { count: MAX_DESCRIPTION_LENGTH })
          : null,
      metric_type: (value) => (value === '' ? i18n.t('metrics.typeRequired') : null),
    },
  });

  const selectedType = form.values.metric_type;
  const aggregationBuilt = isMetricType(selectedType)
    ? buildAggregation(selectedType, aggregationDraft)
    : null;
  const attributionBuilt = buildAttribution(attributionDraft);

  const handleSubmit = (values: CreateMetricFormValues): void => {
    if (isPending || !isMetricType(values.metric_type)) {
      return;
    }
    const aggregation = buildAggregation(values.metric_type, aggregationDraft);
    const attribution = buildAttribution(attributionDraft);
    if (aggregation.aggregation === null || attribution.attribution === null) {
      return;
    }
    const description = values.description.trim();
    onSubmit({
      key: values.key.trim(),
      name: values.name.trim(),
      ...(description.length > 0 ? { description } : {}),
      metric_type: values.metric_type,
      aggregation: aggregation.aggregation,
      attribution: attribution.attribution,
    });
  };

  const handleClose = (): void => {
    if (!isPending) {
      form.reset();
      setAggregationDraft(emptyAggregationDraft());
      setAttributionDraft({ requireExposure: true, windowDaysRaw: '7', fallback: 'subject' });
      onClose();
    }
  };

  return (
    <Modal opened={opened} onClose={handleClose} title={t('metrics.createMetric')} centered size="lg">
      <form onSubmit={form.onSubmit(handleSubmit)} noValidate>
        <Stack gap="md">
          <TextInput
            label={t('metrics.key')}
            placeholder={t('metrics.keyPlaceholder')}
            withAsterisk
            disabled={isPending}
            {...form.getInputProps('key')}
          />
          <TextInput
            label={t('metrics.name')}
            placeholder={t('metrics.namePlaceholder')}
            withAsterisk
            disabled={isPending}
            {...form.getInputProps('name')}
          />
          <Textarea
            label={t('metrics.description')}
            placeholder={t('metrics.descriptionPlaceholder')}
            disabled={isPending}
            autosize
            minRows={2}
            {...form.getInputProps('description')}
          />
          <Select
            label={t('metrics.type')}
            placeholder={t('metrics.typePlaceholder')}
            withAsterisk
            disabled={isPending}
            data={METRIC_TYPES.map((metricType) => ({
              value: metricType,
              label: t(`metrics.types.${metricType}`),
            }))}
            {...form.getInputProps('metric_type')}
            onChange={(next) => {
              form.setFieldValue('metric_type', (next ?? '') as MetricType | '');
              setAggregationDraft(emptyAggregationDraft());
            }}
          />
          {isMetricType(selectedType) ? (
            <AggregationEditor
              metricType={selectedType}
              draft={aggregationDraft}
              error={aggregationBuilt?.error ?? null}
              disabled={isPending}
              onChange={setAggregationDraft}
            />
          ) : null}
          <AttributionEditor
            draft={attributionDraft}
            windowError={attributionBuilt.error === 'windowDays'}
            fallbackError={attributionBuilt.error === 'fallback'}
            disabled={isPending}
            onChange={setAttributionDraft}
          />
          <Group justify="flex-end" gap="sm">
            <Button variant="subtle" onClick={handleClose} disabled={isPending}>
              {t('metrics.cancel')}
            </Button>
            <Button
              type="submit"
              loading={isPending}
              disabled={
                isPending ||
                !form.isValid() ||
                aggregationBuilt?.aggregation === null ||
                attributionBuilt.attribution === null
              }
            >
              {t('metrics.create')}
            </Button>
          </Group>
        </Stack>
      </form>
    </Modal>
  );
}
