package audit

import (
	"context"
	"log/slog"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type OtelEmitter struct {
	serviceName string
	logger      *slog.Logger
}

func NewOtelEmitter(serviceName string) *OtelEmitter {
	return &OtelEmitter{
		serviceName: serviceName,
		logger:      slog.Default().With("component", "audit-otel", "service", serviceName),
	}
}

func (e *OtelEmitter) EmitTrace(ctx context.Context, eventName string, attributes map[string]interface{}) error {
	e.logger.InfoContext(ctx, "trace", "event", eventName, "attrs", attributes)
	return nil
}

func (e *OtelEmitter) EmitMetric(ctx context.Context, metricName string, value float64, attributes map[string]interface{}) error {
	e.logger.InfoContext(ctx, "metric", "name", metricName, "value", value, "attrs", attributes)
	return nil
}

func (e *OtelEmitter) EmitLog(ctx context.Context, level, message string, attributes map[string]interface{}) error {
	e.logger.InfoContext(ctx, "log", "level", level, "msg", message, "attrs", attributes)
	return nil
}

func (e *OtelEmitter) Close() error {
	return nil
}

type MultiEmitter struct {
	emitters []OtelEmitter
}

func NewMultiEmitter(emitters ...OtelEmitter) *MultiEmitter {
	return &MultiEmitter{emitters: emitters}
}

func (e *MultiEmitter) EmitTrace(ctx context.Context, eventName string, attributes map[string]interface{}) error {
	var lastErr error
	for _, em := range e.emitters {
		if err := em.EmitTrace(ctx, eventName, attributes); err != nil {
			lastErr = apperrors.WrapInternalf(err, "emit trace to emitter")
		}
	}
	return lastErr
}

func (e *MultiEmitter) EmitMetric(ctx context.Context, metricName string, value float64, attributes map[string]interface{}) error {
	var lastErr error
	for _, em := range e.emitters {
		if err := em.EmitMetric(ctx, metricName, value, attributes); err != nil {
			lastErr = apperrors.WrapInternalf(err, "emit metric to emitter")
		}
	}
	return lastErr
}

func (e *MultiEmitter) EmitLog(ctx context.Context, level, message string, attributes map[string]interface{}) error {
	var lastErr error
	for _, em := range e.emitters {
		if err := em.EmitLog(ctx, level, message, attributes); err != nil {
			lastErr = apperrors.WrapInternalf(err, "emit log to emitter")
		}
	}
	return lastErr
}
