//go:build integration || manual

package test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/amemiya02/hmdp-go/internal/global"
)

func TestMain(m *testing.M) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	err := global.Init(ctx)
	cancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	code := m.Run()
	if err := global.Close(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		code = 1
	}
	os.Exit(code)
}
