package views

import "os"

func devReloadEnabled() bool {
	return os.Getenv("DEV_RELOAD") == "1"
}
