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

func NextUTCMidnight(now time.Time) time.Time {
	utc := now.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day()+1, 0, 0, 0, 0, time.UTC)
}

func CutoffTimestamp(now time.Time, days int) int64 {
	utc := now.UTC()
	cutoff := time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -days)
	return cutoff.Unix()
}

func Start() {
	if !config.IsMasterNode {
		logger.Log.Infof("log cleanup skipped on slave node")
		return
	}

	days := RetentionDays()
	if days == 0 {
		logger.Log.Infof("log cleanup disabled by LOG_CLEAN_DAYS=0")
		return
	}

	cleanup := func() {
		cutoff := CutoffTimestamp(time.Now(), days)
		count, err := model.DeleteOldLog(cutoff)
		if err != nil {
			logger.Log.Errorf("failed to clean old logs: %v", err)
			return
		}
		logger.Log.Infof("cleaned %d old logs with retention %d days", count, days)
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
