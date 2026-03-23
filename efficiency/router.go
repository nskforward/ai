package efficiency

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/nskforward/ai/llm"
)

// ComplexityLevel уровень сложности
type ComplexityLevel string

const (
	ComplexitySimple   ComplexityLevel = "simple"
	ComplexityModerate ComplexityLevel = "moderate"
	ComplexityComplex  ComplexityLevel = "complex"
)

// ComplexityClassifier классифицирует сложность
type ComplexityClassifier interface {
	// Classify возвращает уровень сложности
	Classify(ctx context.Context, messages []llm.Message) (ComplexityLevel, error)
}

// ModelRouter маршрутизирует запросы по сложности
type ModelRouter interface {
	// Route определяет модель для запроса
	Route(ctx context.Context, messages []llm.Message) (string, error)

	// RegisterClassifier регистрирует классификатор
	RegisterClassifier(classifier ComplexityClassifier)
}

// ModelMapping маппинг сложности на модель
type ModelMapping struct {
	Simple   string
	Moderate string
	Complex  string
}

// DefaultModelMapping возвращает маппинг по умолчанию
func DefaultModelMapping() *ModelMapping {
	return &ModelMapping{
		Simple:   "openai/gpt-4o-mini",
		Moderate: "openai/gpt-4o",
		Complex:  "openai/gpt-4o",
	}
}

// RouterConfig конфигурация роутера
type RouterConfig struct {
	// Mapping маппинг сложности на модель
	Mapping *ModelMapping

	// DefaultModel модель по умолчанию
	DefaultModel string

	// ClassifierModel модель для классификации
	ClassifierModel string
}

// DefaultRouterConfig возвращает конфигурацию по умолчанию
func DefaultRouterConfig() *RouterConfig {
	return &RouterConfig{
		Mapping:         DefaultModelMapping(),
		DefaultModel:    "openai/gpt-4o-mini",
		ClassifierModel: "openai/gpt-4o-mini",
	}
}

// SmartRouter умный роутер с классификацией
type SmartRouter struct {
	config     *RouterConfig
	classifier ComplexityClassifier
	llm        llm.LLM
	mu         sync.RWMutex
}

// NewSmartRouter создаёт новый умный роутер
func NewSmartRouter(provider llm.LLM, config *RouterConfig) *SmartRouter {
	if config == nil {
		config = DefaultRouterConfig()
	}

	router := &SmartRouter{
		config: config,
		llm:    provider,
	}

	// Устанавливаем классификатор по умолчанию
	router.classifier = NewLLMClassifier(provider, config.ClassifierModel)

	return router
}

// Route определяет модель для запроса
func (r *SmartRouter) Route(ctx context.Context, messages []llm.Message) (string, error) {
	r.mu.RLock()
	classifier := r.classifier
	config := r.config
	r.mu.RUnlock()

	if classifier == nil {
		return config.DefaultModel, nil
	}

	// Классифицируем сложность
	complexity, err := classifier.Classify(ctx, messages)
	if err != nil {
		// В случае ошибки используем модель по умолчанию
		return config.DefaultModel, nil
	}

	// Выбираем модель на основе сложности
	switch complexity {
	case ComplexitySimple:
		return config.Mapping.Simple, nil
	case ComplexityModerate:
		return config.Mapping.Moderate, nil
	case ComplexityComplex:
		return config.Mapping.Complex, nil
	default:
		return config.DefaultModel, nil
	}
}

// RegisterClassifier регистрирует классификатор
func (r *SmartRouter) RegisterClassifier(classifier ComplexityClassifier) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.classifier = classifier
}

// LLMClassifier классификатор на основе LLM
type LLMClassifier struct {
	llm   llm.LLM
	model string
	mu    sync.RWMutex
}

// NewLLMClassifier создаёт новый LLM классификатор
func NewLLMClassifier(provider llm.LLM, model string) *LLMClassifier {
	if model == "" {
		model = "openai/gpt-4o-mini"
	}

	return &LLMClassifier{
		llm:   provider,
		model: model,
	}
}

// Classify классифицирует сложность запроса
func (c *LLMClassifier) Classify(ctx context.Context, messages []llm.Message) (ComplexityLevel, error) {
	c.mu.RLock()
	llmProvider := c.llm
	model := c.model
	c.mu.RUnlock()

	if llmProvider == nil {
		return ComplexityModerate, fmt.Errorf("LLM provider not set")
	}

	// Формируем промпт для классификации
	var conversationBuilder strings.Builder
	for _, msg := range messages {
		if msg.Role == llm.RoleUser {
			conversationBuilder.WriteString(fmt.Sprintf("User: %s\n", msg.Content))
		} else if msg.Role == llm.RoleAssistant {
			conversationBuilder.WriteString(fmt.Sprintf("Assistant: %s\n", msg.Content))
		}
	}

	prompt := fmt.Sprintf(`Classify the complexity of this conversation. Respond with ONLY one word: "simple", "moderate", or "complex".

Guidelines:
- simple: Basic questions, greetings, simple facts, single-step tasks
- moderate: Multi-step tasks, code generation, analysis, moderate reasoning
- complex: Complex reasoning, multi-file code changes, research, creative writing, advanced math

Conversation:
%s

Classification:`, conversationBuilder.String())

	req := &llm.GenerateRequest{
		Messages: []llm.Message{
			{
				Role:    llm.RoleUser,
				Content: prompt,
			},
		},
		Model:       model,
		Temperature: 0.1,
		MaxTokens:   10,
	}

	resp, err := llmProvider.Generate(ctx, req)
	if err != nil {
		return ComplexityModerate, fmt.Errorf("classification failed: %w", err)
	}

	// Парсим ответ
	classification := strings.TrimSpace(strings.ToLower(resp.Content))
	switch classification {
	case "simple":
		return ComplexitySimple, nil
	case "moderate":
		return ComplexityModerate, nil
	case "complex":
		return ComplexityComplex, nil
	default:
		return ComplexityModerate, nil
	}
}

// RuleBasedClassifier классификатор на основе правил
type RuleBasedClassifier struct {
	rules []ClassificationRule
	mu    sync.RWMutex
}

// ClassificationRule правило классификации
type ClassificationRule struct {
	// Keywords ключевые слова для匹配
	Keywords []string

	// Complexity уровень сложности
	Complexity ComplexityLevel

	// Weight вес правила
	Weight int
}

// NewRuleBasedClassifier создаёт новый классификатор на основе правил
func NewRuleBasedClassifier() *RuleBasedClassifier {
	classifier := &RuleBasedClassifier{
		rules: make([]ClassificationRule, 0),
	}

	// Добавляем правила по умолчанию
	classifier.addDefaultRules()

	return classifier
}

// Classify классифицирует сложность запроса
func (c *RuleBasedClassifier) Classify(ctx context.Context, messages []llm.Message) (ComplexityLevel, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	scores := map[ComplexityLevel]int{
		ComplexitySimple:   0,
		ComplexityModerate: 0,
		ComplexityComplex:  0,
	}

	// Анализируем все пользовательские сообщения
	for _, msg := range messages {
		if msg.Role != llm.RoleUser {
			continue
		}

		content := strings.ToLower(msg.Content)

		for _, rule := range c.rules {
			for _, keyword := range rule.Keywords {
				if strings.Contains(content, strings.ToLower(keyword)) {
					scores[rule.Complexity] += rule.Weight
				}
			}
		}
	}

	// Определяем максимальный score
	maxScore := 0
	result := ComplexityModerate

	for level, score := range scores {
		if score > maxScore {
			maxScore = score
			result = level
		}
	}

	return result, nil
}

// AddRule добавляет правило классификации
func (c *RuleBasedClassifier) AddRule(rule ClassificationRule) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rules = append(c.rules, rule)
}

// addDefaultRules добавляет правила по умолчанию
func (c *RuleBasedClassifier) addDefaultRules() {
	// Simple правила
	c.rules = append(c.rules, ClassificationRule{
		Keywords:   []string{"hello", "hi", "hey", "thanks", "thank you", "ok", "okay", "yes", "no"},
		Complexity: ComplexitySimple,
		Weight:     1,
	})

	c.rules = append(c.rules, ClassificationRule{
		Keywords:   []string{"what is", "who is", "when is", "where is", "how to"},
		Complexity: ComplexitySimple,
		Weight:     1,
	})

	// Moderate правила
	c.rules = append(c.rules, ClassificationRule{
		Keywords:   []string{"explain", "describe", "compare", "list", "write", "create", "generate"},
		Complexity: ComplexityModerate,
		Weight:     2,
	})

	c.rules = append(c.rules, ClassificationRule{
		Keywords:   []string{"code", "function", "program", "script", "implement"},
		Complexity: ComplexityModerate,
		Weight:     2,
	})

	// Complex правила
	c.rules = append(c.rules, ClassificationRule{
		Keywords:   []string{"analyze", "research", "design", "architect", "optimize", "refactor"},
		Complexity: ComplexityComplex,
		Weight:     3,
	})

	c.rules = append(c.rules, ClassificationRule{
		Keywords:   []string{"complex", "advanced", "sophisticated", "comprehensive", "detailed"},
		Complexity: ComplexityComplex,
		Weight:     3,
	})
}

// KeywordClassifier простой классификатор по ключевым словам
type KeywordClassifier struct {
	keywordMap map[string]ComplexityLevel
	mu         sync.RWMutex
}

// NewKeywordClassifier создаёт новый классификатор по ключевым словам
func NewKeywordClassifier() *KeywordClassifier {
	classifier := &KeywordClassifier{
		keywordMap: make(map[string]ComplexityLevel),
	}

	// Добавляем ключевые слова по умолчанию
	classifier.addDefaultKeywords()

	return classifier
}

// Classify классифицирует сложность запроса
func (c *KeywordClassifier) Classify(ctx context.Context, messages []llm.Message) (ComplexityLevel, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	scores := map[ComplexityLevel]int{
		ComplexitySimple:   0,
		ComplexityModerate: 0,
		ComplexityComplex:  0,
	}

	for _, msg := range messages {
		if msg.Role != llm.RoleUser {
			continue
		}

		content := strings.ToLower(msg.Content)
		words := strings.Fields(content)

		for _, word := range words {
			if complexity, exists := c.keywordMap[word]; exists {
				scores[complexity]++
			}
		}
	}

	// Определяем максимальный score
	maxScore := 0
	result := ComplexityModerate

	for level, score := range scores {
		if score > maxScore {
			maxScore = score
			result = level
		}
	}

	return result, nil
}

// AddKeyword добавляет ключевое слово
func (c *KeywordClassifier) AddKeyword(keyword string, complexity ComplexityLevel) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.keywordMap[strings.ToLower(keyword)] = complexity
}

// addDefaultKeywords добавляет ключевые слова по умолчанию
func (c *KeywordClassifier) addDefaultKeywords() {
	// Simple
	simpleWords := []string{"hi", "hello", "thanks", "ok", "yes", "no", "what", "who", "when", "where"}
	for _, word := range simpleWords {
		c.keywordMap[word] = ComplexitySimple
	}

	// Moderate
	moderateWords := []string{"explain", "describe", "write", "create", "code", "function", "how"}
	for _, word := range moderateWords {
		c.keywordMap[word] = ComplexityModerate
	}

	// Complex
	complexWords := []string{"analyze", "research", "design", "optimize", "complex", "advanced"}
	for _, word := range complexWords {
		c.keywordMap[word] = ComplexityComplex
	}
}
