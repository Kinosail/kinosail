//go:build !unix

package mediaprobe

func lockProbeFixturePublication() func() {
	return func() {}
}
