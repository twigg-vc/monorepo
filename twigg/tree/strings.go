package tree

import "fmt"

func symlinkString(target string) string {
	return fmt.Sprintf("Symlink to: %s", target)
}
