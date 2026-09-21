package config

type ShutdownConfig struct {
	GracePeriod        Duration `json:"gracePeriod" yaml:"gracePeriod" validate:"gt=0"`
	PersistenceTimeout Duration `json:"persistenceTimeout" yaml:"persistenceTimeout" validate:"gt=0"`
}
