package scenario

import (
	"context"
	"net/http"
)

type Scenario interface {
	Name() string
	Run(ctx context.Context, client *http.Client, baseURL string) error
}
