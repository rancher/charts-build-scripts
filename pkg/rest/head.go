package rest

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/rancher/charts-build-scripts/pkg/logger"
)

const maxRetries = 5

// Head sends a HEAD request to the given URL and returns an error if the request fails
func Head(ctx context.Context, url, token string) error {

	// Retry until max retries reached
	for retries := 0; retries <= maxRetries; retries++ {

		// Create a new HEAD request
		req, err := http.NewRequest("HEAD", url, nil)
		if err != nil {
			return fmt.Errorf("error creating the HEAD request: %v", err)
		}

		// Add the authorization header if a token is provided
		if token != "" {
			req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
		}

		// Create a new HTTP client
		client := &http.Client{}

		// Send the request
		response, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("error sending the HEAD request: %v", err)
		}

		// Check the response status code
		switch response.StatusCode {
		case http.StatusOK:
			requestRemaining := response.Header.Get("X-RateLimit-Remaining")
			if requestRemaining != "" {
				logger.Log(ctx, slog.LevelDebug, "request remaining", slog.String("requestRemaining", requestRemaining))
			}
			return nil

		case http.StatusTooManyRequests:
			// Get the retry after header, parse it to int64 and set a timer until retry
			retryAfterMillis := response.Header.Get("Retry-After")
			retryAfterMillisInt, _ := strconv.ParseInt(retryAfterMillis, 10, 64)

			// Convert the retry after millis to a time.Time object and calculate the time until retry
			retryAfter := time.Unix(retryAfterMillisInt, 0)
			timeUntilRetry := time.Until(retryAfter)

			logger.Log(ctx, slog.LevelWarn, "request was rate limited", slog.String("timeUntilRetry", timeUntilRetry.String()))
			time.Sleep(timeUntilRetry)

			continue

		case http.StatusNotFound:
			logger.Log(ctx, slog.LevelDebug, "resource not found", slog.String("url", url))
			return fmt.Errorf("not found")

		default:
			if response.StatusCode >= 500 && response.StatusCode < 600 {
				logger.Log(ctx, slog.LevelWarn, "received server error, trying again in 5 seconds", slog.String("status", response.Status))
				time.Sleep(5 * time.Second)
				continue
			}

			return fmt.Errorf("received unexpected status code %d with message: %v", response.StatusCode, response.Body)
		}
	}

	logger.Log(ctx, slog.LevelError, "max retries reached")
	return fmt.Errorf("max retries reached: %v", maxRetries)
}
