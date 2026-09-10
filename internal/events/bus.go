package events

// Handler receives events published on a Bus.
type Handler func(Event)

// Bus is a minimal synchronous publish/subscribe hub. Handlers run in the
// order they subscribed, on the publishing goroutine.
type Bus struct {
	handlers []Handler
}

// NewBus returns an empty bus.
func NewBus() *Bus { return &Bus{} }

// Subscribe registers h to receive every published event.
func (b *Bus) Subscribe(h Handler) { b.handlers = append(b.handlers, h) }

// Publish delivers e to every subscriber.
func (b *Bus) Publish(e Event) {
	for _, h := range b.handlers {
		h(e)
	}
}
