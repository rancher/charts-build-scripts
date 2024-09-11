package rest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Post sends a POST request to the given URL with the given body and decodes the response into the given response model.
func Post(url string, body, responseModel any) error {

	// Marshal the body
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("error marshalling the body: %v", err)
	}

	// Create a new POST request
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("error creating the POST request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Create a new HTTP client
	client := &http.Client{
		Timeout: time.Second * 2,
	}

	// Send the request
	response, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending the POST request: %v", err)
	}
	defer response.Body.Close()

	// Check the status code
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("received unexpected status code %d with message: %v", response.StatusCode, response.Body)
	}

	// Decode the response body
	err = json.NewDecoder(response.Body).Decode(responseModel)
	if err != nil {
		return fmt.Errorf("error decoding the response body: %v", err)
	}

	return nil
}

// Get sends a GET request to the given URL and returns the response body as a byte slice.
func Get(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status code %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}
