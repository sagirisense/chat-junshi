//go:build !windows

package evidence

func retryableReplace(err error) bool { return false }
