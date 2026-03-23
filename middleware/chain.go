package middleware

// MiddlewareChain управляет цепочкой middleware
type MiddlewareChain struct {
	middlewares []Middleware
}

// NewMiddlewareChain создаёт новую цепочку
func NewMiddlewareChain(middlewares ...Middleware) *MiddlewareChain {
	return &MiddlewareChain{
		middlewares: middlewares,
	}
}

// Then создаёт финальный обработчик из цепочки
func (mc *MiddlewareChain) Then(handler Handler) Handler {
	if handler == nil {
		panic("middleware: handler cannot be nil")
	}

	// Применяем middleware в обратном порядке (последний добавленный - первый выполняется)
	for i := len(mc.middlewares) - 1; i >= 0; i-- {
		handler = mc.middlewares[i](handler)
	}

	return handler
}

// Append добавляет middleware в конец цепочки
func (mc *MiddlewareChain) Append(m ...Middleware) {
	mc.middlewares = append(mc.middlewares, m...)
}

// Prepend добавляет middleware в начало
func (mc *MiddlewareChain) Prepend(m ...Middleware) {
	mc.middlewares = append(m, mc.middlewares...)
}

// Len возвращает количество middleware в цепочке
func (mc *MiddlewareChain) Len() int {
	return len(mc.middlewares)
}
