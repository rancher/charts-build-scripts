package rest

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
)

const maxRetries = 5

// Head sends a HEAD request to the given URL and returns an error if the request fails
func Head(url, token string) error {

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
			logrus.Infof("request remaining: %s", requestRemaining)

			return nil

		case http.StatusTooManyRequests:
			// Get the retry after header, parse it to int64 and set a timer until retry
			retryAfterMillis := response.Header.Get("Retry-After")
			retryAfterMillisInt, _ := strconv.ParseInt(retryAfterMillis, 10, 64)

			// Convert the retry after millis to a time.Time object and calculate the time until retry
			retryAfter := time.Unix(retryAfterMillisInt, 0)
			timeUntilRetry := time.Until(retryAfter)

			logrus.Infof("request was rate limited, retrying in %s", timeUntilRetry)
			time.Sleep(timeUntilRetry)

			continue

		case http.StatusNotFound:
			return fmt.Errorf("not found")

		default:
			return fmt.Errorf("received unexpected status code %d with message: %v", response.StatusCode, response.Body)
		}
	}

	return fmt.Errorf("max retries reached: %v", maxRetries)
}
