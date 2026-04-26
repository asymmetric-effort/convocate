package user

import (
	"fmt"
	"os"
)

// GetuidFn is overridable in tests so we can simulate running as a different
// uid without actually switching users.
var GetuidFn = os.Getuid

// EnforceRunningAs refuses to proceed unless the current process runs as the
// Linux user with the given name. Returns a friendly, self-contained error
// that tells the operator exactly how to invoke the binary correctly. This
// is called at the top of main for both convocate and convocate-agent —
// neither tool is safe to run as any other uid because on-disk paths
// (~/.claude, /home/claude/.ssh/authorized_keys, session dirs) assume the
// claude user owns them.
func EnforceRunningAs(username string) error {
	want, err := Lookup(username)
	if err != nil {
		return fmt.Errorf("enforce user: %w", err)
	}
	got := GetuidFn()
	if got == want.UID {
		return nil
	}
	return fmt.Errorf("must run as user %q (uid %d), currently uid %d — "+
		"try: sudo -u %s %s", username, want.UID, got, username, os.Args[0])
}
