package service

func response(requestID string, payload map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"request_id": requestID,
		"type":       "RESPONSE",
		"payload":    payload,
	}
}

func failure(requestID, code, message string) map[string]interface{} {
	return map[string]interface{}{
		"request_id": requestID,
		"type":       "ERROR",
		"payload": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	}
}
