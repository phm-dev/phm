package httputil

import (
	"net/http"
	"time"
)

var Client = &http.Client{Timeout: 60 * time.Second}
