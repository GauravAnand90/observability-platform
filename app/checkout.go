package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/GauravAnand90/observability-platform/app/internal/fault"
)

var tracer = otel.Tracer("github.com/GauravAnand90/observability-platform/app")

var errPaymentGateway = errors.New("payment gateway returned 502 Bad Gateway")

// checkoutHandler simulates an order flow with three child spans. Injected
// faults land in payment.authorize, so in Tempo the failing or slow step is
// obvious at a glance: that's the point of correlating SLO alerts with traces.
func checkoutHandler(faults *fault.Settings) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		orderID := fmt.Sprintf("ord-%08x", rand.Uint32())
		trace.SpanFromContext(ctx).SetAttributes(attribute.String("app.order_id", orderID))

		work(ctx, "inventory.reserve", 5*time.Millisecond, 25*time.Millisecond)

		if err := authorizePayment(ctx, faults); err != nil {
			span := trace.SpanFromContext(ctx)
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			writeJSON(w, http.StatusBadGateway, map[string]string{"order_id": orderID, "error": err.Error()})
			return
		}

		work(ctx, "order.persist", 5*time.Millisecond, 20*time.Millisecond)
		writeJSON(w, http.StatusOK, map[string]string{"order_id": orderID, "status": "confirmed"})
	})
}

func authorizePayment(ctx context.Context, faults *fault.Settings) error {
	_, span := tracer.Start(ctx, "payment.authorize", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()
	span.SetAttributes(attribute.String("peer.service", "payment-gateway"))

	extra := faults.ExtraLatency()
	if extra > 0 {
		span.SetAttributes(attribute.Int64("app.injected_latency_ms", extra.Milliseconds()))
	}
	time.Sleep(jitter(20*time.Millisecond, 60*time.Millisecond) + extra)

	if faults.ShouldFail() {
		span.RecordError(errPaymentGateway)
		span.SetStatus(codes.Error, errPaymentGateway.Error())
		return errPaymentGateway
	}
	return nil
}

func work(ctx context.Context, name string, min, max time.Duration) {
	_, span := tracer.Start(ctx, name)
	defer span.End()
	time.Sleep(jitter(min, max))
}

func jitter(min, max time.Duration) time.Duration {
	return min + time.Duration(rand.Int64N(int64(max-min)+1))
}
