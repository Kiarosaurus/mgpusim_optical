package optical

import (
	"fmt"
	"log"

	"github.com/sarchlab/akita/v4/sim"
)

// *Luz llegando al otro extremo*
type LinkDeliveryEvent struct {
	*sim.EventBase
	Msg     sim.Msg
	DstPort sim.Port
}

func NewLinkDeliveryEvent(time sim.VTimeInSec, handler sim.Handler, msg sim.Msg, dst sim.Port) *LinkDeliveryEvent {
	return &LinkDeliveryEvent{
		EventBase: sim.NewEventBase(time, handler),
		Msg:       msg,
		DstPort:   dst,
	}
}

// Entrega física en el BUFFER de entrada DEL DESTINO.
func (e *LinkDeliveryEvent) Execute(engine sim.Engine) error {
	e.DstPort.Deliver(e.Msg)
	return nil
}

// Fibra Óptica.
type Link struct {
	*sim.TickingComponent

	SideA   sim.Port
	SideB   sim.Port
	Latency sim.VTimeInSec
}

// TODO. Justo ahora la latencia es estática y el bandwidth infinito.
func NewLink(name string, engine sim.Engine, latency sim.VTimeInSec) *Link {
	l := &Link{
		TickingComponent: sim.NewTickingComponent(name, engine, 1*sim.GHz, nil),
		Latency:          latency,
	}
	return l
}

func (l *Link) PlugIn(port sim.Port) {

	fmt.Printf("[DEBUG_PLUGIN] Link '%s' connected to Port '%s' (Pointer: %p).\n", l.Name(), port.Name(), port)

	if l.SideA == nil {
		l.SideA = port
	} else if l.SideB == nil {
		l.SideB = port
	} else {
		log.Panicf("OpticalLink %s supports only 2 ports.", l.Name())
	}

	port.SetConnection(l)
}

func (l *Link) Unplug(port sim.Port) {
	if l.SideA == port {
		l.SideA = nil
	} else if l.SideB == port {
		l.SideB = nil
	}
}

func (l *Link) NotifyAvailable(port sim.Port) {
}

// Llamado cuando alguien quiere enviar algo.
func (l *Link) NotifySend() {
	now := l.Engine.CurrentTime()

	// Revisando si hay mensajes pendientes de salir.
	// En Akita v4 el Link debe buscar activamente quién quiere enviar.
	if l.SideA != nil {
		l.CheckAndForward(now, l.SideA, l.SideB)
	}
	if l.SideB != nil {
		l.CheckAndForward(now, l.SideB, l.SideA)
	}
}

// --- LÓGICA DE DESEMPAQUETADO ---
func (l *Link) CheckAndForward(now sim.VTimeInSec, src, dst sim.Port) {
	msg := src.RetrieveOutgoing() // Extrayendo el mensaje del puerto origen.

	if msg == nil {
		return
	}

	if dst == nil {
		fmt.Printf("[LINK_ERROR] %s: Delivery attempt failed (destination not connected).\n", l.Name())
		return
	}

	msgToDeliver := msg

	// Si el mensaje viene del Switch (sobre), lo abrimos.
	if pkt, ok := msg.(*OpticalPacket); ok {
		msgToDeliver = pkt.InnerMsg
	} else { // Viene de la GPU (mensaje normal), lo pasamos tal cual.
		fmt.Printf("[LINK_TRAFFIC] %s: %s -> %s (Type: %T)\n", l.Name(), src.Name(), dst.Name(), msg)
	}

	// La siguiente llegada se basa en la latencia de la fibra.
	evt := NewLinkDeliveryEvent(now+l.Latency, l, msgToDeliver, dst)
	l.Engine.Schedule(evt)
}

// Para que sea componente...
func (l *Link) Handle(e sim.Event) error {
	switch evt := e.(type) {
	case *LinkDeliveryEvent:
		return evt.Execute(l.Engine)
	default:
		return l.TickingComponent.Handle(e)
	}
}
