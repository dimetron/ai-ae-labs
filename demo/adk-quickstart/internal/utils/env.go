// Package utils містить допоміжні утиліти для роботи з оточенням та конфігурацією.
package utils

import (
	"os"
	"path/filepath"
	"strings"
)

// LoadDotEnv завантажує змінні з файлу .env (або вказаних шляхів) в оточення os.Setenv,
// якщо вони ще не були встановлені.
//
// Без аргументів шукає спочатку локальний ./.env, а потім піднімається вгору по
// дереву каталогів до першого apps/.env — так само, як internal/adkenv у лабах.
// Завдяки цьому демо, запущене зі своєї теки, бачить спільний apps/.env репозиторію.
// З явними шляхами читає лише їх. Відсутні файли пропускаються без помилки.
//
// Явний export у терміналі завжди виграє у файлу: вже наявні змінні не перезаписуються.
func LoadDotEnv(paths ...string) {
	if len(paths) == 0 {
		paths = defaultPaths()
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

// defaultPaths повертає кандидатів для пошуку конфігурації, коли явні шляхи не
// вказані: локальний ./.env та найближчий apps/.env вище по дереву каталогів.
func defaultPaths() []string {
	var paths []string

	if _, err := os.Stat(".env"); err == nil {
		paths = append(paths, ".env")
	}
	if path, ok := findAppsEnv(); ok {
		paths = append(paths, path)
	}

	return paths
}

// findAppsEnv піднімається від поточної робочої теки до кореня файлової системи
// й повертає шлях до першого знайденого apps/.env.
func findAppsEnv() (string, bool) {
	dir, err := os.Getwd()
	if err != nil {
		return "", false
	}

	for {
		candidate := filepath.Join(dir, "apps", ".env")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, true
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}
