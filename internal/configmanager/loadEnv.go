// configmamanger load/init env vars
package configmanager

import "os"

func initEnv() {
	envPath := os.Getenv("KNOV_DATA_PATH")
	if envPath != "" {
		DataPath = envPath
	}
}
