package config

type ShutdownConfig struct {
	GracePeriod        Duration `json:"gracePeriod"`
	PersistenceTimeout Duration `json:"persistenceTimeout"`
}
