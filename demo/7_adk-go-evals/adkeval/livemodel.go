package adkeval

import (
	"context"
	"fmt"

	"github.com/dimetron/pi-go/pimodels"
	"google.golang.org/adk/v2/model"
)

// LiveModel будує справжню модель через каталог models.dev (той самий шлях,
// що й demo/adk-quickstart): gemini-3.7-flash за замовчуванням. Потрібен
// GEMINI_API_KEY (або відповідний ключ провайдера). Повертається model.LLM,
// який годиться і для агента, і для LLM-as-judge.
func LiveModel(ctx context.Context, name string) (model.LLM, error) {
	if name == "" {
		name = "gemini-3.7-flash"
	}
	m, err := pimodels.New(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("live model %s: %w (перевірте API-ключ провайдера)", name, err)
	}
	return m, nil
}
