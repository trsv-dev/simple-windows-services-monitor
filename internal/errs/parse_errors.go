package errs

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ParseServiceError ServiceControlError Парсит ошибку из вывода sc команды.
func ParseServiceError(output string) *ServiceError {
	output = strings.TrimSpace(output)

	// Ищем паттерн "FAILED XXXX:"
	re := regexp.MustCompile(`FAILED\s+(\d+):`)
	matches := re.FindStringSubmatch(output)

	if len(matches) > 1 {
		code, _ := strconv.Atoi(matches[1])
		msg := ParseErrorCode(code)

		return NewServiceError(msg, code)
	}

	// Если FAILED нет, но есть другая ошибка
	if strings.Contains(output, "ERROR") {
		code := -1
		msg := "Unknown service error"

		return NewServiceError(msg, code)
	}

	return nil
}

// ParseErrorCode Преобразует код ошибки Windows в понятное описание.
func ParseErrorCode(code int) string {
	errorMap := map[int]string{
		1:    "Invalid function",
		2:    "File not found",
		5:    "Access denied",
		87:   "Invalid parameter",
		1051: "A stop control has been sent to a service that other running services are dependent on",
		1052: "The requested control is not valid for this service",
		1058: "The service cannot be started, either because it is disabled or because it has no enabled devices associated with it.",
		1060: "The specified service does not exist as an installed service",
		1061: "The service cannot accept control messages at this time",
		1062: "The service has not been started",
		1063: "The service process could not connect to the service controller",
		1064: "An exception occurred in the service when handling the control request",
		1065: "The service did not set the status field to pending",
		1066: "The service status is currently pending",
		1067: "The service did not report a status from the ServiceMain function within the timeout interval",
		1068: "The process terminated unexpectedly",
		1069: "No attempt has been made to start the service since the last boot",
		1070: "The service database is locked",
		1071: "A default service is being removed from the database",
		1072: "The current boot has already been accepted for service database recovery",
		1073: "No recovery actions could be found for this service",
		1074: "The database is in use by the notify request subscriber",
		1075: "The service is being removed from the database",
		1076: "The service has already been flagged for removal from the database",
		1077: "The current service is being removed from the database",
		1078: "The service could not be deleted from the database",
		1079: "Unable to open service manager database",
		1080: "An error occurred when calling a Windows function",
		1083: "The dependency cycle was detected when opening a service",
		1084: "Unable to find the service name in the database",
		1100: "The event log is full",
		1101: "Event logging disabled",
		1102: "The event log file has changed between read operations",
	}

	if msg, exists := errorMap[code]; exists {
		return msg
	}
	return fmt.Sprintf("Windows error code %d", code)
}
