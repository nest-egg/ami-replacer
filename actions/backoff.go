package actions

import (
	"time"
	"github.com/cenkalti/backoff"
)

func newExponentialBackOff() *backoff.ExponentialBackOff {
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = time.Duration(10) * time.Second
	b.MaxInterval = time.Duration(30) * time.Second
	b.MaxElapsedTime = time.Duration(300) * time.Second
	b.Reset()
	return b
}

func newShortExponentialBackOff() *backoff.ExponentialBackOff {
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = time.Duration(1) * time.Second
	b.MaxInterval = time.Duration(10) * time.Second
	b.MaxElapsedTime = time.Duration(600) * time.Second
	b.Reset()
	return b
}
