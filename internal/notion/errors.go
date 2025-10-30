package notion

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// Error represents a structured error returned by the Notion API.
type Error struct {
	Message string `json:"message"`
	Code    string `json:"code"`
	Status  int    `json:"status"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("notion: %s (code=%s status=%d)", e.Message, e.Code, e.Status)
}

// decodeError attempts to materialize a Notion error from a non-2xx HTTP response.
func decodeError(resp *http.Response) error {
	if resp == nil {
		return errors.New("notion: nil response")
	}

	body, readErr := io.ReadAll(resp.Body)
	closeErr := resp.Body.Close()
	if readErr != nil {
		joined := errors.Join(readErr, closeErr)
		return fmt.Errorf("notion: read error response: %w", joined)
	}
	if closeErr != nil {
		return fmt.Errorf("notion: close error response: %w", closeErr)
	}

	var ne Error
	if err := json.Unmarshal(body, &ne); err != nil {
		return &Error{
			Status:  resp.StatusCode,
			Code:    resp.Status,
			Message: string(body),
		}
	}

	if ne.Status == 0 {
		ne.Status = resp.StatusCode
	}
	if ne.Code == "" {
		ne.Code = resp.Status
	}
	return &ne
}
