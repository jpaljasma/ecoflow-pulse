package main

import (
	"context"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflow"
)

type quotaPollKind string

const (
	quotaPollKindFallback  quotaPollKind = "fallback"
	quotaPollKindReconcile quotaPollKind = "reconcile"
	quotaPollKindLiveness  quotaPollKind = "liveness"
)

type quotaPollRequest struct {
	Kind       quotaPollKind
	Timeout    time.Duration
	Requested  time.Time
	SilenceAge time.Duration
}

type quotaPollResult struct {
	Request  quotaPollRequest
	Quota    map[string]string
	Error    error
	Duration time.Duration
}

func runQuotaPollWorker(
	ctx context.Context,
	service *ecoflow.GeneralInfoService,
	deviceSN string,
	requests <-chan quotaPollRequest,
	results chan<- quotaPollResult,
) {
	for {
		select {
		case <-ctx.Done():
			return
		case req, ok := <-requests:
			if !ok {
				return
			}
			start := time.Now()
			pollCtx, pollCancel := context.WithTimeout(ctx, req.Timeout)
			quota, _, err := service.GetDeviceAllQuota(pollCtx, deviceSN)
			pollCancel()
			result := quotaPollResult{
				Request:  req,
				Quota:    quota,
				Error:    err,
				Duration: time.Since(start),
			}
			select {
			case <-ctx.Done():
				return
			case results <- result:
			}
		}
	}
}
