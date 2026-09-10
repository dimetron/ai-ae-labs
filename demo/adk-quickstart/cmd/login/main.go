// Command login здійснює вхід через OpenAI Codex Device Flow або OAuth PKCE
// та зберігає отриманий API-токен у локальний файл .env.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/dimetron/ai-eng-course/demo/adk-quickstart/internal/auth"
)

func main() {
	ctx := context.Background()

	providerFlag := flag.String("provider", "codex", "OAuth/Device провайдер (codex | opencode)")
	flag.Parse()

	providerName := *providerFlag
	if flag.NArg() > 0 {
		providerName = flag.Arg(0)
	}

	prov, ok := auth.FindProvider(providerName)
	if !ok {
		log.Fatalf("Провайдер %q не знайдений (доступні: codex, opencode)", providerName)
	}

	fmt.Printf("==> Авторизація через %s...\n", prov.Name)

	if prov.CodexDeviceAuth {
		sess, err := auth.StartCodexDeviceFlow(ctx, prov)
		if err != nil {
			log.Fatalf("Помилка запуску device flow: %v", err)
		}

		fmt.Printf("\n1. Відкрийте у браузері: %s\n", sess.VerificationURL)
		fmt.Printf("2. Введіть код підтвердження: %s\n\n", sess.UserCode)
		fmt.Println("Очікування підтвердження у браузері...")

		result, err := auth.CompleteCodexDeviceFlow(ctx, prov, sess)
		if err != nil {
			log.Fatalf("Помилка завершення авторизації: %v", err)
		}
		if result.Err != nil {
			log.Fatalf("Авторизація не вдалася: %v", result.Err)
		}

		saveResult(result)
		return
	}

	result, err := auth.PKCEFlow(ctx, prov, func(url string) error {
		fmt.Printf("Відкрийте посилання у браузері: %s\n", url)
		return nil
	})
	if err != nil {
		log.Fatalf("Помилка PKCE: %v", err)
	}
	if result.Err != nil {
		log.Fatalf("Авторизація не вдалася: %v", result.Err)
	}

	saveResult(result)
}

func saveResult(result *auth.Result) {
	fmt.Printf("\n✅ Успішний вхід! Отримано токен для %s.\n", result.EnvVar)

	// Зберігаємо у локальний файл .env
	data, _ := os.ReadFile(".env")
	updated := auth.UpdateEnvVar(string(data), result.EnvVar, result.APIKey)
	if err := os.WriteFile(".env", []byte(updated), 0600); err == nil {
		fmt.Printf("Токен автоматично збережено у файл .env (%s=...)\n", result.EnvVar)
	} else {
		_ = auth.SaveKey(result.EnvVar, result.APIKey)
		fmt.Printf("Встановіть змінну оточення вручну:\nexport %s=%q\n", result.EnvVar, result.APIKey)
	}
}
