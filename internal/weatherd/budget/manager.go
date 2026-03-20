package budget

import (
	"sync"
	"time"
)

type Config struct {
	DailyLimit     int
	PerMinuteLimit int
	NowFn          func() time.Time
}

type Snapshot struct {
	DailyLimit     int
	DailyUsed      int
	PerMinuteLimit int
	MinuteUsed     int
	DayStart       time.Time
	MinuteStart    time.Time
}

type Manager struct {
	mu             sync.Mutex
	dailyLimit     int
	perMinuteLimit int
	nowFn          func() time.Time
	dayStart       time.Time
	minuteStart    time.Time
	dailyUsed      int
	minuteUsed     int
}

func New(cfg Config) *Manager {
	nowFn := cfg.NowFn
	if nowFn == nil {
		nowFn = time.Now
	}
	return &Manager{
		dailyLimit:     cfg.DailyLimit,
		perMinuteLimit: cfg.PerMinuteLimit,
		nowFn:          nowFn,
	}
}

func (m *Manager) Allow(cost int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.nowFn().UTC()
	m.resetWindows(now)
	if cost <= 0 {
		return true
	}
	if m.dailyLimit > 0 && m.dailyUsed+cost > m.dailyLimit {
		return false
	}
	if m.perMinuteLimit > 0 && m.minuteUsed+cost > m.perMinuteLimit {
		return false
	}
	m.dailyUsed += cost
	m.minuteUsed += cost
	return true
}

func (m *Manager) Snapshot() Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.nowFn().UTC()
	m.resetWindows(now)
	return Snapshot{
		DailyLimit:     m.dailyLimit,
		DailyUsed:      m.dailyUsed,
		PerMinuteLimit: m.perMinuteLimit,
		MinuteUsed:     m.minuteUsed,
		DayStart:       m.dayStart,
		MinuteStart:    m.minuteStart,
	}
}

func (m *Manager) resetWindows(now time.Time) {
	dayStart := now.Truncate(24 * time.Hour)
	if m.dayStart.IsZero() || !m.dayStart.Equal(dayStart) {
		m.dayStart = dayStart
		m.dailyUsed = 0
	}
	minuteStart := now.Truncate(time.Minute)
	if m.minuteStart.IsZero() || !m.minuteStart.Equal(minuteStart) {
		m.minuteStart = minuteStart
		m.minuteUsed = 0
	}
}
