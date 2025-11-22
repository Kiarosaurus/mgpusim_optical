package optical

import (
	"fmt"
	"log"
	"sync"

	"github.com/sarchlab/akita/v4/mem/mem"
	"github.com/sarchlab/akita/v4/sim"
)

// --- DEFINICIÓN DEL WRAPPER ---

// Wrapper para los mensajes de Akita.
// Motivo: Akita fuerza que los componentes envíen mensajes firmados por otros.
// Este Wrapper nos ayuda a que el Switch firme el paquete.
type OpticalPacket struct {
	sim.MsgMeta
	InnerMsg sim.Msg // Mensaje.
}

func (p *OpticalPacket) Meta() *sim.MsgMeta {
	return &p.MsgMeta
}

// Clone es necesario para cumplir la interfaz sim.Msg de Akita v4.
func (p *OpticalPacket) Clone() sim.Msg {
	newPacket := *p
	newPacket.ID = sim.GetIDGenerator().Generate()
	// Clonamos el mensaje interno para evitar race condition.
	if p.InnerMsg != nil {
		newPacket.InnerMsg = p.InnerMsg.Clone()
	}
	return &newPacket
}

// --- DEFINICIÓN DEL SWITCH ---

type Switch struct {
	*sim.TickingComponent

	Latency sim.VTimeInSec

	// Mapea ID de DESTINO (String) -> Puerto de SALIDA (Objeto).
	// TODO. Que sea modificada dinámicamente por el Controller.
	RouteTable map[sim.RemotePort]sim.Port

	connectedPorts []sim.Port

	// Recolecta  bytes enviados entre pares (Source -> Destination).
	// TODO. Que la use el predictor.
	TrafficMatrix map[string]map[string]uint64
	matrixLock    sync.Mutex

	// TODO.
	// Parte de RECONFIGURACIÓN.
}

func NewSwitch(name string, engine sim.Engine) *Switch {
	s := &Switch{
		TickingComponent: sim.NewTickingComponent(name, engine, 1*sim.GHz, nil),
		Latency:          40 * 1e-9, // (~40ns según Flexfly).
		RouteTable:       make(map[sim.RemotePort]sim.Port),
		TrafficMatrix:    make(map[string]map[string]uint64),
	}
	return s
}

func (s *Switch) CreatePort(name string) sim.Port {
	// TODO.
	// He puesto un buffer enorme pero debería buscar para switches ópticos.
	port := sim.NewPort(s, 4096, 4096, name)
	s.connectedPorts = append(s.connectedPorts, port)
	return port
}

// Configuración de la tabla de enrutamiento estática inicial.
func (s *Switch) RegisterDestination(dstName sim.RemotePort, outputPort sim.Port) {
	s.RouteTable[dstName] = outputPort
}

// Punto input de mensajes desde los Links.
func (s *Switch) NotifyRecv(port sim.Port) {
	now := s.Engine.CurrentTime()

	req := port.RetrieveIncoming()
	if req == nil {
		return
	}

	s.ProcessMsg(now, req)
}

func (s *Switch) NotifyPortFree(port sim.Port) {
	// TODO. Para control de flujo.
}

// --- LÓGICA DEL SWITCH ---
func (s *Switch) ProcessMsg(now sim.VTimeInSec, msg sim.Msg) {
	// TODO. Verificar en caso haya RECONFIGURACIÓN.
	// Podríamos descartar paquetes o ponerlos en buffers.

	dst := msg.Meta().Dst
	src := msg.Meta().Src

	// 1. Registrar Tráfico (para el predictor).
	s.RecordTraffic(now, src, dst, msg)

	// 2. Enrutar.
	outPort, found := s.RouteTable[dst]
	if !found { // RouteTable incompleta o destino inexistente.
		log.Panicf("[OPTICAL_SWITCH] Panic: No route to %s from %s", dst, src)
	}

	// 3. Verificando congestión de la salida.
	if !outPort.CanSend() {
		fmt.Printf("[OPTICAL_SWITCH] DROP! Port %s full sending to %s\n", outPort.Name(), dst)
		return
	}

	// 4. Reenviar (+ encapsulamiento)
	packet := &OpticalPacket{
		MsgMeta: sim.MsgMeta{
			ID:  sim.GetIDGenerator().Generate(), // Generamos ID válido
			Src: outPort.AsRemote(),              // Switch firma como remitente.
			Dst: dst,                             // Destino final se mantiene para el Link.
			// TODO.
			TrafficBytes: 16, // Overhead del header óptico.
			TrafficClass: "OpticalPacket",
		},
		InnerMsg: msg,
	}
	// Eliminada la línea packet.SendTime = now (no existe en v4)

	outPort.Send(packet)
}

// --- EXTRACIÓN DE MÉTRICAS (tráfico) ---
func (s *Switch) RecordTraffic(now sim.VTimeInSec, src, dst sim.RemotePort, msg sim.Msg) {
	s.matrixLock.Lock()
	defer s.matrixLock.Unlock()

	size := uint64(1)
	typeStr := "Unknown"

	switch m := msg.(type) {
	case *mem.ReadReq:
		size = 8 // Overhead. TODO.
		typeStr = "ReadReq"
	case *mem.WriteReq:
		size = uint64(len(m.Data))
		typeStr = "WriteReq"
	case *mem.DataReadyRsp:
		size = uint64(len(m.Data))
		typeStr = "DataReady"
	case *mem.WriteDoneRsp:
		typeStr = "WriteDone"
	}

	srcName := string(src)
	dstName := string(dst)

	// Inicialización del mapa.
	if _, ok := s.TrafficMatrix[srcName]; !ok {
		s.TrafficMatrix[srcName] = make(map[string]uint64)
	}
	s.TrafficMatrix[srcName][dstName] += size

	fmt.Printf("[SWITCH_MONITOR] Time: %.6f | %s (%s) -> %s | Size: %d bytes\n",
		now, typeStr, src, dst, size)
}

// Hace que el Switch sea un componente de simulación estándar.
func (s *Switch) Handle(e sim.Event) error {
	return s.TickingComponent.Handle(e)
}
