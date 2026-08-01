package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"navo/internal/logstore"
)

func (s *Service) handleLogsQuery(requestID string, msg map[string]interface{}) map[string]interface{} {
	from, err := optionalLogTime(msg["from"])
	if err != nil {
		return errorResponse(requestID, "INVALID_ARGUMENT", fmt.Errorf("from: %w", err))
	}
	to, err := optionalLogTime(msg["to"])
	if err != nil {
		return errorResponse(requestID, "INVALID_ARGUMENT", fmt.Errorf("to: %w", err))
	}
	if !from.IsZero() && !to.IsZero() && from.After(to) {
		return errorResponse(requestID, "INVALID_ARGUMENT", fmt.Errorf("from must not be after to"))
	}
	levels := make([]logstore.Level, 0)
	for _, value := range logStringList(msg["levels"]) {
		level := logstore.Level(strings.ToUpper(value))
		switch level {
		case logstore.LevelDebug, logstore.LevelInfo, logstore.LevelWarn, logstore.LevelError:
			levels = append(levels, level)
		default:
			return errorResponse(requestID, "INVALID_ARGUMENT", fmt.Errorf("unsupported log level %q", value))
		}
	}
	afterID := logNumberAsInt(msg["after_id"])
	limit := logNumberAsInt(msg["limit"])
	result := logstore.Default().Query(logstore.Query{
		Levels: levels, Services: logStringList(msg["services"]),
		From: from, To: to, AfterID: uint64(max(afterID, 0)), Limit: limit,
	})
	return response(requestID, map[string]interface{}{
		"entries": result.Entries, "next_cursor": result.NextCursor, "has_more": result.HasMore,
	})
}

func logNumberAsInt(value any) int {
	switch number := value.(type) {
	case int:
		return number
	case float64:
		return int(number)
	case json.Number:
		parsed, _ := number.Int64()
		return int(parsed)
	default:
		return 0
	}
}

func optionalLogTime(value any) (time.Time, error) {
	text, _ := value.(string)
	if strings.TrimSpace(text) == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse(time.RFC3339, text)
	if err != nil {
		return time.Time{}, err
	}
	return parsed.UTC(), nil
}

func logStringList(value any) []string {
	switch values := value.(type) {
	case []string:
		return values
	case []interface{}:
		result := make([]string, 0, len(values))
		for _, value := range values {
			if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
				result = append(result, text)
			}
		}
		return result
	default:
		return nil
	}
}

func logServiceForMethod(method string) string {
	switch {
	case strings.HasPrefix(method, "tun."), strings.HasPrefix(method, "capture."), strings.HasPrefix(method, "network."):
		return "TUN"
	case strings.HasPrefix(method, "subscription."):
		return "Subscription"
	case strings.HasPrefix(method, "ip."):
		return "IPDetection"
	case strings.HasPrefix(method, "metrics."):
		return "NetworkMonitor"
	case strings.HasPrefix(method, "core."):
		return "Service"
	default:
		return "Service"
	}
}
