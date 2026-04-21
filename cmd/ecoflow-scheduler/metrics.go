package main

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type schedulerMetrics struct {
	jobRuns           *prometheus.CounterVec
	jobDuration       *prometheus.HistogramVec
	cleanupRows       *prometheus.CounterVec
	lastSuccessfulRun *prometheus.GaugeVec
	retainedRows      *prometheus.GaugeVec
}

func newSchedulerMetrics(registerer prometheus.Registerer) *schedulerMetrics {
	if registerer == nil {
		registerer = prometheus.DefaultRegisterer
	}
	m := &schedulerMetrics{
		jobRuns: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "pulse_scheduler_jobs_total",
			Help: "Scheduler job executions by job type and result.",
		}, []string{"job_type", "result"}),
		jobDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "pulse_scheduler_job_duration_seconds",
			Help:    "Scheduler job execution duration by job type and result.",
			Buckets: prometheus.DefBuckets,
		}, []string{"job_type", "result"}),
		cleanupRows: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "pulse_scheduler_cleanup_rows_total",
			Help: "Rows compacted or pruned by the scheduler cleanup jobs.",
		}, []string{"job_type", "kind"}),
		lastSuccessfulRun: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pulse_scheduler_last_successful_run_unixtime",
			Help: "Unix time of the latest successful scheduler job run by job type.",
		}, []string{"job_type"}),
		retainedRows: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pulse_scheduler_retained_rows",
			Help: "Current retained forecast rows by logical table or kind.",
		}, []string{"kind"}),
	}
	registerer.MustRegister(
		m.jobRuns,
		m.jobDuration,
		m.cleanupRows,
		m.lastSuccessfulRun,
		m.retainedRows,
	)
	return m
}

func (m *schedulerMetrics) observeJobRun(jobType string, err error, duration time.Duration, finishedAt time.Time) {
	if m == nil {
		return
	}
	result := "success"
	if err != nil {
		result = "error"
	}
	m.jobRuns.WithLabelValues(jobType, result).Inc()
	m.jobDuration.WithLabelValues(jobType, result).Observe(duration.Seconds())
	if err == nil {
		m.lastSuccessfulRun.WithLabelValues(jobType).Set(float64(finishedAt.UTC().Unix()))
	}
}

func (m *schedulerMetrics) observeCleanupRows(jobType, kind string, rows int64) {
	if m == nil || rows <= 0 {
		return
	}
	m.cleanupRows.WithLabelValues(jobType, kind).Add(float64(rows))
}

func (m *schedulerMetrics) setRetainedRows(kind string, rows int64) {
	if m == nil {
		return
	}
	m.retainedRows.WithLabelValues(kind).Set(float64(rows))
}
