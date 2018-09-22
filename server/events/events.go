package events

type Handler struct {
	Inbound chan Event
}

type Event struct {
}

func (h *Handler) Run() {
	for {
		h.tick()
	}
}

func (h *Handler) tick() {
	select {
	case e := <-h.Inbound:
	}
}
