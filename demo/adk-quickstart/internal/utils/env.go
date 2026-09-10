// Package utils містить допоміжні утиліти для роботи з оточенням та конфігурацією.
package utils

import (
	"os"
	"strings"
)

// LoadDotEnv завантажує змінні з файлу .env (або вказаних шляхів) в оточення os.Setenv,
// якщо вони ще не були встановлені.
func LoadDotEnv(paths ...string) {
	if len(paths) == 0 {
		paths = []string{".env"}
	}

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}

			key, val, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}

			key = strings.TrimSpace(key)
			val = strings.Trim(strings.TrimSpace(val), `"'`)

			if _, exists := os.LookupEnv(key); !exists && key != "" {
				_ = os.Setenv(key, val)
			}
		}
	}
}
