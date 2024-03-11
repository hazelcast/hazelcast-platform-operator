package controller

import "time"

type ReconcilerOption struct {
	RetryAfter time.Duration
	Err        error
}

func RetryAfter(dur time.Duration) ReconcilerOption {
	return ReconcilerOption{RetryAfter: dur}
}

func Error(err error) ReconcilerOption {
	return ReconcilerOption{Err: err}
}

func Empty() ReconcilerOption {
	return ReconcilerOption{}
}
