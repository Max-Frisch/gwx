package web

import "os"

func lookupEnv(key string) string { return os.Getenv(key) }
