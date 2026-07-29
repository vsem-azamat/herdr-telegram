package herdr_test

import (
	"context"
	"errors"
	"time"

	"github.com/vsem-azamat/herdr-telegram/herdr"
)

func ExampleClient_Snapshot() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := herdr.NewClient("/path/to/herdr.sock")
	if err != nil {
		return
	}
	_, err = client.Snapshot(ctx)
	var apiErr *herdr.APIError
	if errors.As(err, &apiErr) {
		_ = apiErr.Code
	}
}

func ExampleClient_Prompt() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := herdr.NewClient("/path/to/herdr.sock")
	if err != nil {
		return
	}
	_, err = client.Prompt(ctx, "w1:p1", "continue", herdr.PromptOptions{})
	var ambiguous *herdr.AmbiguousPromptError
	if errors.As(err, &ambiguous) {
		// The server may have accepted the prompt. Do not retry automatically.
		return
	}
}
