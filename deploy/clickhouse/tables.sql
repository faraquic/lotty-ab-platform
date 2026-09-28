CREATE DATABASE IF NOT EXISTS labp;

CREATE TABLE IF NOT EXISTS labp.events_raw(
    event_id UUID,
    event_type LowCardinality(String),
    subject_id String,
    salt_version LowCardinality(String) DEFAULT '',
    occurred_at DateTime64(3, 'UTC'),
    received_at DateTime64(3, 'UTC'),
    decision_id UUID DEFAULT '00000000-0000-0000-0000-000000000000',
    payload String,
    schema_version UInt16
)
ENGINE = ReplacingMergeTree(received_at)
PARTITION BY toYYYYMM(occurred_at)
ORDER BY (event_id)
TTL toDateTime(occurred_at) + INTERVAL 90 DAY;

CREATE TABLE IF NOT EXISTS labp.decisions_raw(
    decision_id UUID,
    request_id UUID,
    subject_id String,
    salt_version LowCardinality(String) DEFAULT '',
    flag_key String,
    experiment_id UUID DEFAULT '00000000-0000-0000-0000-000000000000',
    experiment_version_id UUID DEFAULT '00000000-0000-0000-0000-000000000000',
    variant_id UUID DEFAULT '00000000-0000-0000-0000-000000000000',
    result_source LowCardinality(String),
    config_revision UInt64,
    created_at DateTime64(3, 'UTC')
)
ENGINE = ReplacingMergeTree(created_at)
PARTITION BY toYYYYMM(created_at)
ORDER BY (decision_id, flag_key)
TTL toDateTime(created_at) + INTERVAL 90 DAY;

CREATE TABLE IF NOT EXISTS labp.exposures(
    event_id UUID,
    decision_id UUID,
    subject_id String,
    salt_version LowCardinality(String) DEFAULT '',
    experiment_id UUID,
    experiment_version_id UUID,
    variant_id UUID,
    flag_id UUID,
    occurred_at DateTime64(3, 'UTC'),
    received_at DateTime64(3, 'UTC')
)
ENGINE = ReplacingMergeTree(received_at)
PARTITION BY toYYYYMM(occurred_at)
ORDER BY (event_id)
TTL toDateTime(occurred_at) + INTERVAL 90 DAY;

CREATE TABLE IF NOT EXISTS labp.attributed_events(
    event_id UUID,
    decision_id UUID,
    experiment_id UUID DEFAULT '00000000-0000-0000-0000-000000000000',
    variant_id UUID DEFAULT '00000000-0000-0000-0000-000000000000',
    subject_id String,
    salt_version LowCardinality(String) DEFAULT '',
    event_type LowCardinality(String),
    occurred_at DateTime64(3, 'UTC'),
    received_at DateTime64(3, 'UTC')
)
ENGINE = ReplacingMergeTree(received_at)
PARTITION BY toYYYYMM(occurred_at)
ORDER BY (event_id)
TTL toDateTime(occurred_at) + INTERVAL 90 DAY;
