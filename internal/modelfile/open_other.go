//go:build !unix

package modelfile

import "os"

// Non-Unix targets retain descriptor checks. This path has compile evidence;
// it does not claim Unix nonblocking-open behavior for other platforms.
func openRead(name string) (*os.File, error) { return os.Open(name) }
