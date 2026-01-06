package worker

import (
	"strconv"
	"time"
)

// Backoff delays in seconds for each attempt (0-indexed)
// Attempt 1 (index 0): immediate (0s)
// Attempt 2 (index 1): +1 minute
// Attempt 3 (index 2): +5 minutes
// Attempt 4 (index 3): +15 minutes
// Attempt 5 (index 4): +1 hour
// Attempt 6 (index 5): +3 hours
// Attempt 7 (index 6): +8 hours
// Attempt 8 (index 7): +24 hours
var backoffDelays = []time.Duration{
	0,                // Attempt 1: immediate
	1 * time.Minute,  // Attempt 2: +1 minute
	5 * time.Minute,  // Attempt 3: +5 minutes
	15 * time.Minute, // Attempt 4: +15 minutes
	1 * time.Hour,    // Attempt 5: +1 hour
	3 * time.Hour,    // Attempt 6: +3 hours
	8 * time.Hour,    // Attempt 7: +8 hours
	24 * time.Hour,   // Attempt 8: +24 hours
}

// CalculateBackoffDelay calculates the delay for the next attempt based on attempt count
// attemptCount is the current attempt count (1-indexed)
// Returns the duration to wait before the next attempt
func CalculateBackoffDelay(attemptCount int) time.Duration {
	// attemptCount is 1-indexed, so we need to convert to 0-indexed for array access
	// For attempt 1, we want index 0 (0s delay)
	// For attempt 2, we want index 1 (1 minute delay)
	index := attemptCount - 1

	// Clamp index to valid range
	if index < 0 {
		index = 0
	}
	if index >= len(backoffDelays) {
		// If we exceed the backoff table, use the last delay (24 hours)
		index = len(backoffDelays) - 1
	}

	return backoffDelays[index]
}

// ParseRetryAfterHeader parses the Retry-After header value
// Returns the duration and true if parsing was successful
// Retry-After can be either a number of seconds (as a string) or an HTTP date
// For simplicity, we'll handle seconds as integer
func ParseRetryAfterHeader(retryAfter string) (time.Duration, bool) {
	if retryAfter == "" {
		return 0, false
	}

	// Try parsing as seconds (integer)
	seconds, err := strconv.Atoi(retryAfter)
	if err == nil {
		if seconds < 0 {
			return 0, false
		}
		return time.Duration(seconds) * time.Second, true
	}

	// TODO: Could also parse HTTP date format, but for now we'll just return false
	// HTTP date format: "Wed, 21 Oct 2015 07:28:00 GMT"
	return 0, false
}
