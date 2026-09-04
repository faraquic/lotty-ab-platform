package logger

const (
	FieldServiceName    = "service.name"
	FieldServiceVersion = "service.version"
	FieldEnvironment    = "environment"
)

const (
	FieldServerAddress   = "server.address"
	FieldClientAddress   = "client.address"
	FieldRequestMethod   = "request.method"
	FieldRequestID       = "request.id"
	FieldRoute           = "route"
	FieldResponseCode    = "response.status_code"
	FieldResponseSize    = "response.body.size"
	FieldDurationMs      = "duration_ms"
	FieldShutdownTimeout = "shutdown_timeout"
)

const (
	FieldErrorType = "error.type"
	FieldPanicMsg  = "panic.message"
	FieldStack     = "stack_trace"
)

const (
	FieldDBSystem = "db.system"
	FieldDBHost   = "db.host"
	FieldDBPort   = "db.port"
	FieldDBName   = "db.name"
	FieldDBStatus = "db.status"
	FieldDBMsg    = "db.message"
)

const (
	FieldCacheSystem    = "cache.system"
	FieldCacheOperation = "cache.operation"
	FieldCacheHit       = "cache.hit"
	FieldCacheStatus    = "cache.status"
	FieldCacheMsg       = "cache.message"
	FieldCacheAddrs     = "cache.addrs"
	FieldCacheDB        = "cache.db"
	FieldCacheKeyNS     = "cache.key_namespace"
)

const (
	FieldStorageSystem    = "storage.system"
	FieldStorageOperation = "storage.operation"
	FieldStoragePrefix    = "storage.object_prefix"
	FieldStorageStatus    = "storage.status"
	FieldStorageMsg       = "storage.message"
	FieldBucket           = "storage.bucket"
	FieldRegion           = "storage.region"
	FieldEndpoint         = "storage.endpoint"
)

const (
	FieldAuthFailure  = "auth.failure_reason"
	FieldAuthTokenSrc = "auth.token_source"
	FieldAuthSessionN = "auth.session_count"
)

const (
	FieldUserID      = "user.id"
	FieldActorID     = "actor.id"
	FieldUserRole    = "user.role"
	FieldUserNewRole = "user.role.new"
	FieldUserOldRole = "user.role.old"
)

const (
	FieldFlagID   = "flag.id"
	FieldFlagKey  = "flag.key"
	FieldFlagType = "flag.type"
)

const (
	FieldMetricID   = "metric.id"
	FieldMetricKey  = "metric.key"
	FieldMetricType = "metric.type"
)

const (
	FieldSnapshotFlagCount = "snapshot.flag_count"
	FieldSnapshotStatus    = "snapshot.status"
	FieldSnapshotMsg       = "snapshot.message"
	FieldQueueLen          = "queue.len"
	FieldQueueCap          = "queue.cap"
)
