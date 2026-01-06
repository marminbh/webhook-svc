package worker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// DeliveryResult represents the result of a webhook delivery attempt
type DeliveryResult struct {
	HTTPStatus      *int
	LatencyMs       int
	ResponseBody    string
	ResponseSummary *string
	Error           error
	RetryAfter      string // Retry-After header value if present
}

// DeliverWebhook performs an HTTP POST request to the webhook URL
func DeliverWebhook(
	url string,
	payload map[string]interface{},
	secret string,
	timeoutSeconds int,
	maxResponseBodySize int,
	logger *zap.Logger,
) *DeliveryResult {
	result := &DeliveryResult{}

	// Marshal payload to JSON
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		result.Error = fmt.Errorf("failed to marshal payload: %w", err)
		return result
	}

	// Generate HMAC signature
	signature, err := GenerateHMACSignature(payloadBytes, secret)
	if err != nil {
		result.Error = fmt.Errorf("failed to generate HMAC signature: %w", err)
		return result
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: time.Duration(timeoutSeconds) * time.Second,
	}

	// Create request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		result.Error = fmt.Errorf("failed to create HTTP request: %w", err)
		return result
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Marmin-Signature", signature)

	// Record start time
	startTime := time.Now()

	// Perform request
	resp, err := client.Do(req)
	if err != nil {
		// Network/timeout error
		latencyMs := int(time.Since(startTime).Milliseconds())
		result.LatencyMs = latencyMs
		result.Error = fmt.Errorf("HTTP request failed: %w", err)
		return result
	}
	defer resp.Body.Close()

	// Calculate latency
	latencyMs := int(time.Since(startTime).Milliseconds())
	result.LatencyMs = latencyMs
	result.HTTPStatus = &resp.StatusCode

	// Read response body (limited to maxResponseBodySize)
	responseBody := make([]byte, maxResponseBodySize+1) // +1 to detect truncation
	n, readErr := io.ReadFull(resp.Body, responseBody)
	if readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
		logger.Warn("Failed to read response body",
			zap.Error(readErr),
			zap.String("url", url),
		)
	}

	// Truncate if necessary
	if n > maxResponseBodySize {
		result.ResponseBody = string(responseBody[:maxResponseBodySize])
		summary := fmt.Sprintf("Response body truncated (read %d bytes, max %d)", n, maxResponseBodySize)
		result.ResponseSummary = &summary
	} else {
		result.ResponseBody = string(responseBody[:n])
		if n > 0 {
			summary := fmt.Sprintf("Response body: %s", result.ResponseBody)
			// Truncate summary if too long
			if len(summary) > 500 {
				summary = summary[:500] + "..."
			}
			result.ResponseSummary = &summary
		}
	}

	// Check for Retry-After header
	if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
		result.RetryAfter = retryAfter
	}

	return result
}
