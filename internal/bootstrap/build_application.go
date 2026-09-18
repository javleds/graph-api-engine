package bootstrap

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	api "github.com/javleds/api-engine/internal/api/http"
	"github.com/javleds/api-engine/internal/execution/application"
	"github.com/javleds/api-engine/internal/execution/domain"
	"github.com/javleds/api-engine/internal/execution/infrastructure/inprocess"
	executionsql "github.com/javleds/api-engine/internal/execution/infrastructure/sqlite"
	fetchdomain "github.com/javleds/api-engine/internal/fetch/domain"
	notificationapp "github.com/javleds/api-engine/internal/notification/application"
	notificationhttp "github.com/javleds/api-engine/internal/notification/infrastructure/http"
	notificationsql "github.com/javleds/api-engine/internal/notification/infrastructure/sqlite"
	"github.com/javleds/api-engine/internal/platform/clock"
	"github.com/javleds/api-engine/internal/platform/config"
	"github.com/javleds/api-engine/internal/platform/identifier"
	"github.com/javleds/api-engine/internal/platform/observability"
	security "github.com/javleds/api-engine/internal/security/infrastructure"
	"github.com/javleds/api-engine/internal/security/infrastructure/egress"
	"github.com/javleds/api-engine/internal/worker"
)

func Build(ctx context.Context, cfg *config.Config) (app *Application, err error) {
	if err = cfg.Validate(); err != nil {
		return nil, err
	}
	var level slog.Level
	if err = level.UnmarshalText([]byte(cfg.Observability.LogLevel)); err != nil {
		return nil, err
	}
	a := &Application{Config: cfg, Metrics: observability.NewMetrics(), Logger: slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level}))}
	defer func() {
		if err != nil {
			a.Close()
		}
	}()
	protector, identity, err := a.openStorage(ctx)
	if err != nil {
		return nil, err
	}
	fetcher, err := a.buildFetch(protector, identity)
	if err != nil {
		return nil, err
	}
	authenticator := &security.TokenAuthenticator{Development: !cfg.Security.AuthenticationEnabled}
	if cfg.Security.AuthenticationEnabled {
		authenticator, err = security.LoadTokenAuthenticator(cfg.Security.PrincipalsFile)
		if err != nil {
			return nil, err
		}
	}
	instance, err := identifier.UUIDGenerator{}.NewID()
	if err != nil {
		return nil, err
	}
	builder := &notificationapp.WebhookPayloadBuilder{
		PublicBaseURL:        cfg.HTTP.PublicBaseURL,
		MaxPayloadBytes:      cfg.Webhooks.MaxPayloadBytes,
		DeliveryMaxAge:       cfg.Webhooks.DeliveryMaxAge.Value(),
		MaxRootCalls:         cfg.Executions.MaxRootCalls,
		MaxErrorMessageBytes: cfg.Executions.Errors.MaxMessageBytes,
	}
	if err = builder.Validate(); err != nil {
		return nil, err
	}
	finalizer := &application.FinalizeExecution{
		Store:   a.Store,
		Writer:  executionsql.NewExecutionFinalizer(a.Store),
		Prepare: builder.PrepareCompletion,
		OnPublished: func(c domain.ExecutionCompletion) {
			a.Metrics.Add("executions_"+string(c.Status)+"_total", 1)
			a.Metrics.Observe("result_size_bytes", float64(len(c.Data)))
			a.Logger.Info("execution terminal", "execution_id", c.ID, "status", c.Status, "node_failures", c.Report.ErrorSummary.Total)
		},
		OnFailure: func(error) { a.ready.Store(false); a.Logger.Error("terminal persistence failed") },
	}
	ids, err := executionsql.InterruptedIDs(ctx, a.Store, instance)
	if err != nil {
		return nil, err
	}
	recovery := application.RecoverExecutions{Store: a.Store, Finalizer: finalizer}
	if err = recovery.Execute(ctx, ids, time.Now().UTC()); err != nil {
		return nil, err
	}
	outbox := notificationsql.NewSQLiteWebhookOutbox(a.EngineDB, protector)
	if err = outbox.Recover(ctx, instance, time.Now()); err != nil {
		return nil, err
	}
	webhookPolicy, err := egress.NewPolicy(cfg.Security.Egress.WebhookAllowedOrigins, cfg.Security.Egress.AllowedPrivateCIDRs, cfg.Security.Egress.AllowHTTPForDevelopment)
	if err != nil {
		return nil, err
	}
	sender := notificationhttp.NewHTTPWebhookSender(webhookPolicy, notificationhttp.SenderOptions{
		MaxResponseBytes: int64(cfg.Webhooks.MaxResponseBytes),
		ConnectTimeout:   cfg.Fetch.ConnectTimeout.Value(),
		AttemptTimeout:   cfg.Webhooks.AttemptTimeout.Value(),
		MaxConcurrent:    cfg.Webhooks.MaxConcurrent,
	})
	a.closeNotificationTransport = sender.Close
	a.Notifications = &notificationapp.WebhookDispatcher{
		Outbox: outbox,
		Sender: measuredWebhookSender{sender: sender, metrics: a.Metrics},
		Clock:  clock.RealClock{},
		Options: notificationapp.DispatcherOptions{
			InstanceID:         instance,
			MaxConcurrent:      cfg.Webhooks.MaxConcurrent,
			PollInterval:       cfg.Webhooks.PollInterval.Value(),
			ClaimLease:         cfg.Webhooks.ClaimLease.Value(),
			AttemptTimeout:     cfg.Webhooks.AttemptTimeout.Value(),
			PersistenceTimeout: cfg.Shutdown.PersistenceTimeout.Value(),
			MaxAttempts:        cfg.Webhooks.MaxAttempts,
			BaseDelay:          cfg.Webhooks.BaseDelay.Value(),
			MaxBackoff:         cfg.Webhooks.MaxBackoff.Value(),
		},
		OnError: func(error) { a.Logger.Error("webhook delivery persistence failed") },
	}
	engine := &worker.Engine{
		Fetcher:     fetcher,
		Progress:    a.Store,
		IdentityKey: identity,
		Options:     workerOptions(cfg),
		FetchPolicy: fetchdomain.FetchPolicy{
			AttemptTimeout:   cfg.Fetch.AttemptTimeout.Value(),
			OperationTimeout: cfg.Deduplication.MaxOperationTime.Value(),
			MaxAttempts:      cfg.Fetch.Retry.MaxAttempts,
			BaseDelay:        cfg.Fetch.Retry.BaseDelay.Value(),
			MaxBackoff:       cfg.Fetch.Retry.MaxBackoff.Value(),
		},
	}
	runner := &worker.ExecutionRunner{Store: a.Store, Finalizer: finalizer, Engine: engine, InstanceID: instance, ResultRetention: cfg.Storage.ResultRetention.Value()}
	a.Dispatcher = inprocess.NewLocalExecutionDispatcher(context.WithoutCancel(ctx), measuredRunner{runner: runner, store: a.Store, metrics: a.Metrics}, cfg.Executions.MaxConcurrent, cfg.Executions.Timeout.Value())
	a.Dispatcher.OnFailure = func(id domain.ExecutionID, _ error) {
		a.Logger.Error("execution supervisor observed failure", "execution_id", id)
		persist, cancel := context.WithTimeout(context.Background(), cfg.Shutdown.PersistenceTimeout.Value())
		defer cancel()
		_, current, err := a.Store.Load(persist, id)
		if err != nil {
			a.ready.Store(false)
			return
		}
		if current.Status.Terminal() {
			return
		}
		err = finalizer.Finish(persist, domain.ExecutionCompletion{
			ID:              id,
			ExpectedVersion: current.Version,
			Status:          domain.Failed,
			Error:           &domain.ExecutionFailure{Code: "INTERNAL_WORKER_ERROR", Message: "The worker could not publish a reliable result."},
			Report:          domain.EmptyReport(),
			FinishedAt:      time.Now().UTC(),
		})
		if err != nil {
			a.ready.Store(false)
		}
	}
	creator := &application.CreateExecution{
		OnCreated:         func(domain.Execution) { a.Metrics.Add("executions_accepted_total", 1) },
		Store:             a.Store,
		Dispatcher:        a.Dispatcher,
		Finalizer:         finalizer,
		IDs:               identifier.UUIDGenerator{},
		Clock:             clock.RealClock{},
		IdentityKey:       identity,
		MetadataRetention: cfg.Storage.MetadataRetention.Value(),
		CheckCapacity:     a.checkCapacity,
	}
	server := &api.Server{
		ValidateQuotas:  a.registerQuotas,
		WebhookStatus:   outbox,
		Create:          creator,
		Reader:          a.Store,
		Results:         a.Store,
		Auth:            authenticator,
		Clock:           clock.RealClock{},
		Limits:          cfg.DefinitionLimits(),
		MaxRequestBytes: int64(cfg.HTTP.MaxRequestBytes),
		MaxJSONDepth:    cfg.Fetch.MaxJSONDepth,
		Ready:           a.isReady,
	}
	if cfg.Observability.MetricsEnabled {
		server.Metrics = a.Metrics
		server.MetricsPath = cfg.Observability.MetricsPath
	}
	a.HTTP = &http.Server{
		Addr:              cfg.HTTP.Address,
		Handler:           a.observeHTTP(server.Handler()),
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout.Value(),
		ReadTimeout:       cfg.HTTP.ReadTimeout.Value(),
		WriteTimeout:      cfg.HTTP.WriteTimeout.Value(),
		IdleTimeout:       cfg.HTTP.IdleTimeout.Value(),
	}
	a.ready.Store(true)
	return a, nil
}
