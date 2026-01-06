package worker

import (
	"fmt"
	"time"
)

// DeliveryStatus represents the status after processing a delivery result
type DeliveryStatus struct {
	Status        string    // 'succeeded', 'pending', or 'failed'
	NextAttemptAt time.Time // When to retry next (if status is 'pending')
	LastError     *string   // Error message if any
}

// ProcessDeliveryResult processes a delivery result and determines the next status
func ProcessDeliveryResult(
	result *DeliveryResult,
	attemptCount int,
	maxAttempts int,
) DeliveryStatus {
	status := DeliveryStatus{}

	// Check if we have an error (network/timeout)
	if result.Error != nil {
		// Network/timeout error - treat as transient
		if attemptCount >= maxAttempts {
			errorMsg := fmt.Sprintf("Max attempts reached: %v", result.Error)
			status.Status = "failed"
			status.LastError = &errorMsg
			return status
		}

		// Calculate backoff for next attempt
		backoffDelay := CalculateBackoffDelay(attemptCount + 1)
		status.Status = "pending"
		status.NextAttemptAt = time.Now().Add(backoffDelay)
		errorMsg := fmt.Sprintf("Network error: %v", result.Error)
		status.LastError = &errorMsg
		return status
	}

	// Check HTTP status code
	if result.HTTPStatus == nil {
		// This shouldn't happen, but handle it
		if attemptCount >= maxAttempts {
			errorMsg := "Max attempts reached: no HTTP status code"
			status.Status = "failed"
			status.LastError = &errorMsg
			return status
		}

		backoffDelay := CalculateBackoffDelay(attemptCount + 1)
		status.Status = "pending"
		status.NextAttemptAt = time.Now().Add(backoffDelay)
		errorMsg := "No HTTP status code received"
		status.LastError = &errorMsg
		return status
	}

	httpStatus := *result.HTTPStatus

	// Success (2xx)
	if httpStatus >= 200 && httpStatus < 300 {
		status.Status = "succeeded"
		return status
	}

	// Rate limited (429)
	if httpStatus == 429 {
		// Check if Retry-After header is present
		if result.RetryAfter != "" {
			retryAfter, ok := ParseRetryAfterHeader(result.RetryAfter)
			if ok && retryAfter > 0 {
				status.Status = "pending"
				status.NextAttemptAt = time.Now().Add(retryAfter)
				errorMsg := fmt.Sprintf("Rate limited (429), retry after %v", retryAfter)
				status.LastError = &errorMsg
				return status
			}
		}

		// No valid Retry-After header, use backoff
		if attemptCount >= maxAttempts {
			errorMsg := fmt.Sprintf("Max attempts reached: rate limited (429)")
			status.Status = "failed"
			status.LastError = &errorMsg
			return status
		}

		backoffDelay := CalculateBackoffDelay(attemptCount + 1)
		status.Status = "pending"
		status.NextAttemptAt = time.Now().Add(backoffDelay)
		errorMsg := fmt.Sprintf("Rate limited (429)")
		status.LastError = &errorMsg
		return status
	}

	// Other errors (4xx, 5xx) - treat as transient
	if attemptCount >= maxAttempts {
		errorMsg := fmt.Sprintf("Max attempts reached: HTTP %d", httpStatus)
		status.Status = "failed"
		status.LastError = &errorMsg
		return status
	}

	// Calculate backoff for next attempt
	backoffDelay := CalculateBackoffDelay(attemptCount + 1)
	status.Status = "pending"
	status.NextAttemptAt = time.Now().Add(backoffDelay)
	errorMsg := fmt.Sprintf("HTTP %d", httpStatus)
	status.LastError = &errorMsg
	return status
}
