package logcleanup

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/songquanpeng/one-api/common/config"
	"github.com/songquanpeng/one-api/common/logger"
	"github.com/songquanpeng/one-api/model"
)

const DefaultRetentionDays = 7
const DefaultBodiesRetentionDays = 3

func RetentionDays() int {
	raw := strings.TrimSpace(os.Getenv("LOG_CLEAN_DAYS"))
	if raw == "" {
		return DefaultRetentionDays
	}

	days, err := strconv.Atoi(raw)
	if err != nil {
		return DefaultRetentionDays
	}
	if days == 0 {
		return 0
	}
	if days < 0 || days > DefaultRetentionDays {
		return DefaultRetentionDays
	}
	return days
}

func BodiesRetentionDays() int {
	raw := strings.TrimSpace(os.Getenv("LOG_CLEAN_BODIES_DAYS"))
	if raw == "" {
		return DefaultBodiesRetentionDays
	}

	days, err := strconv.Atoi(raw)
	if err != nil {
		return DefaultBodiesRetentionDays
	}
	if days == 0 {
		return 0
	}
	if days < 0 || days > DefaultBodiesRetentionDays {
		return DefaultBodiesRetentionDays
	}
	return days
}

func NextUTCMidnight(now time.Time) time.Time {
	utc := now.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day()+1, 0, 0, 0, 0, time.UTC)
}

func CutoffTimestamp(now time.Time, days int) int64 {
	utc := now.UTC()
	cutoff := time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -days)
	return cutoff.Unix()
}

func scheduleDaily(name string, days int, task func(cutoff int64) (int64, error)) {
	if days == 0 {
		logger.Log.Infof("%s cleanup disabled (retention days = 0)", name)
		return
	}

	cleanup := func() {
		cutoff := CutoffTimestamp(time.Now(), days)
		count, err := task(cutoff)
		if err != nil {
			logger.Log.Errorf("failed to clean old %s: %v", name, err)
			return
		}
		logger.Log.Infof("cleaned %d old %s with retention %d days", count, name, days)
	}

	cleanup()
	go func() {
		for {
			if sleepUntil := time.Until(NextUTCMidnight(time.Now())); sleepUntil > 0 {
				time.Sleep(sleepUntil)
			}
			cleanup()
		}
	}()
}

func Start() {
	if !config.IsMasterNode {
		logger.Log.Infof("log cleanup skipped on slave node")
		return
	}

	scheduleDaily("logs", RetentionDays(), func(cutoff int64) (int64, error) {
		return model.DeleteOldLog(cutoff)
	})

	scheduleDaily("log bodies", BodiesRetentionDays(), func(cutoff int64) (int64, error) {
		return model.ClearOldLogBodies(cutoff)
	})
}
