package bootstrap

import (
	"context"
	"database/sql"
	definition "github.com/javleds/api-engine/internal/definition/domain"
	"github.com/javleds/api-engine/internal/execution/infrastructure/inprocess"
	executionsql "github.com/javleds/api-engine/internal/execution/infrastructure/sqlite"
	fetchhttp "github.com/javleds/api-engine/internal/fetch/infrastructure/http"
	fetchsql "github.com/javleds/api-engine/internal/fetch/infrastructure/sqlite"
	notificationapp "github.com/javleds/api-engine/internal/notification/application"
	"github.com/javleds/api-engine/internal/platform/config"
	"github.com/javleds/api-engine/internal/platform/observability"
	platformsql "github.com/javleds/api-engine/internal/platform/sqlite"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
)

type Application struct {
	handlers                   sync.WaitGroup
	handlerMu                  sync.Mutex
	handlersClosed             bool
	registerQuotas             func(context.Context, definition.CompiledDefinition) error
	activeFlights              func() int
	Config                     *config.Config
	HTTP                       *http.Server
	Dispatcher                 *inprocess.LocalExecutionDispatcher
	Notifications              *notificationapp.WebhookDispatcher
	EngineDB                   *sql.DB
	CacheDB                    *sql.DB
	Lock                       *platformsql.InstanceLock
	Store                      *executionsql.ExecutionStore
	Cache                      *fetchsql.SQLiteResponseCache
	Transport                  *fetchhttp.NetHTTPTransport
	Metrics                    *observability.Metrics
	Logger                     *slog.Logger
	closeNotificationTransport func()
	ready                      atomic.Bool
}

func (a *Application) Close() {
	if a.closeNotificationTransport != nil {
		a.closeNotificationTransport()
	}
	if a.Transport != nil {
		a.Transport.CloseIdleConnections()
	}
	if a.EngineDB != nil {
		a.EngineDB.Close()
	}
	if a.CacheDB != nil {
		a.CacheDB.Close()
	}
	if a.Lock != nil {
		a.Lock.Close()
	}
}
