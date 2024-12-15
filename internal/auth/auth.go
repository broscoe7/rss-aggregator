package auth

import (
	"errors"
	"net/http"
	"strings"
)

// Extracts the API Key from HTTP request headers
func GetApiKey(headers http.Header) (string, error) {
	val := headers.Get("Authorization")
	if val == "" {
		return "", errors.New("no authentication info found")
	}
	vals := strings.Split(val, " ")
	if len(vals) != 2 || vals[0] != "ApiKey" {
		return "", errors.New("invalid authorization header")
	}
	return vals[1], nil
}