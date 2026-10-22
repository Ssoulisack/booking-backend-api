package outsrc

import (
	"backend/pkg/logger"
	"bytes"
	"fmt"

	"io/ioutil"

	"net/http"
)

type CustomHTTPClient struct {
	Client *http.Client
}

type HttpRequest interface {
	Do(log_key string, req *http.Request) (*http.Response, error)
}

// NewCustomHTTPClient creates a new instance of CustomHTTPClient.
func NewCustomHTTPClient(Client *http.Client) HttpRequest {
	return &CustomHTTPClient{
		Client: Client,
	}
}

var log = logger.NewLogger()

func (c *CustomHTTPClient) Do(log_key string, req *http.Request) (*http.Response, error) {
	log.Info(fmt.Sprintf("key:%v Method %v : %v", log_key, req.Method, req.URL))
	// Log the request payload
	if req.Body != nil {
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Info(fmt.Sprintf("Error reading request body: %v", err))
		} else {
			log.Info(fmt.Sprintf("key:%v Request: %s", log_key, body))
			// Reset the request body for subsequent use
			req.Body = ioutil.NopCloser(bytes.NewBuffer(body))
		}
	}
	req.Close = true
	// Send the request and get the response
	resp, err := c.Client.Do(req)

	// Log the response for non-GET and non-OPTIONS requests
	if err != nil {
		log.Error(fmt.Sprintf("Error sending request: %v", err), err, nil)
	} else if req.Method != "GET" && req.Method != "OPTIONS" {
		defer resp.Body.Close()
		responseBody, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Info(fmt.Sprintf("key:%s Error reading response body: %v", log_key, err))
		} else {
			log.Info(fmt.Sprintf("key:%v Response: %s", log_key, responseBody))
			resp.Body = ioutil.NopCloser(bytes.NewBuffer(responseBody))
		}
		log.Info(fmt.Sprintf("REQUEST_PATH# '%v', RESPONSE_STATUS_CODE# '%v' , RESPONSE# '%v'", resp.Request.URL.RawPath, resp.StatusCode, string(responseBody)))
	}

	return resp, err
}
