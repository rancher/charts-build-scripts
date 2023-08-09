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
func Head(url string) error {

	// Retry until max retries reached
	for retries := 0; retries <= maxRetries; retries++ {

		// Create a new HEAD request
		req, err := http.NewRequest("HEAD", url, nil)
		if err != nil {
			return fmt.Errorf("error creating the HEAD request: %v", err)
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
			retryAfter := time.Now().Add(time.Duration(retryAfterMillisInt) * time.Nanosecond)
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
