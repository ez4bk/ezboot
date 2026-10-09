package retry

import (
	"time"

	"github.com/cenkalti/backoff/v4"
)

const DefaultTimes = 3

func Exponential(times int, f func() error) error {
	_, err := ExponentialExt(times, func() (any, error) {
		return nil, f()
	})
	return err
}

func Constant(times int, f func() error, interval time.Duration) error {
	_, err := ConstantExt(times, func() (any, error) {
		return nil, f()
	}, interval)
	return err
}

func ExponentialExt[T any](times int, f func() (T, error)) (res T, err error) {
	err = withBackoff(times, func() error {
		res, err = f()
		return err
	}, backoff.NewExponentialBackOff())
	return res, err
}

func ConstantExt[T any](times int, f func() (T, error), interval time.Duration) (res T, err error) {
	err = withBackoff(times, func() error {
		res, err = f()
		return err
	}, backoff.NewConstantBackOff(interval))
	return res, err
}

func withBackoff(times int, f func() error, b backoff.BackOff) (err error) {
	return backoff.Retry(func() error {
		if times--; times >= 0 {
			err = f()
			return err
		}
		return backoff.Permanent(err)
	}, b)
}
