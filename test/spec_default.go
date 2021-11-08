//go:build !ee && !os
// +build !ee,!os

package test

func IsEE() bool {
	return false
}
