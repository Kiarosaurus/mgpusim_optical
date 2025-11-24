package optical

import (
	"fmt"
	"log"
	"sync"

	"github.com/sarchlab/akita/v4/mem/mem"
	"github.com/sarchlab/akita/v4/sim"
)

// --- DEFINICIÓN DEL WRAPPER ---

// Wrapper para los mensajes de Akita en el Switch.
// Motivo: Akita fuerza a que quien envía tiene que ser el firmante.
// Este Wrapper nos ayuda a que el Switch firme el paquete.
type OpticalPacket struct {
	sim.MsgMeta         // Metadata (ID, Src, Dst).
	InnerMsg    sim.Msg // Mensaje real.
}

// Requisito de interfaz sim.Msg
func (p *OpticalPacket) Meta() *sim.MsgMeta {
	return &p.MsgMeta
}

// Requisito de interfaz sim.Msg
func (p *OpticalPacket) Clone() sim.Msg {
	newPacket := *p
	newPacket.ID = sim.GetIDGenerator().Generate()
	// Clonamos el mensaje interno para (evitar race condition).
	if p.InnerMsg != nil {
		newPacket.InnerMsg = p.InnerMsg.Clone()
	}
	return &newPacket
}

// --- DEFINICIÓN DEL COMPONENTE SWITCH ---

type Switch struct {
	*sim.TickingComponent // Habilitando un reloj. ES COMPONENTE.

	Latency sim.VTimeInSec // Habilitando Latencia.

	// Mapea ID de DESTINO (String) -> Puerto de SALIDA (Objeto).
	// TODO. Que sea modificada dinámicamente por el Controller.
	RouteTable map[sim.RemotePort]sim.Port

	connectedPorts []sim.Port

	// Recolectamos quién habla con quién (Source -> Destination -> Bytes).
	// TODO. Que la use el predictor.
	TrafficMatrix map[string]map[string]uint64
	matrixLock    sync.Mutex // Lock. La usará el Switch (escribir) + Controller (leer).

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

// --- ¿BUFFER ÓPTICO? ---

// En Anderson et al. y Zhu et al. los switches ópticos son Bufferless.

// Tiempo de reconfiguración (Anderson et al.): 820 ns.
// Ancho de banda: 32 GB/s.	 <- En PCIe 4 se usan 16 lanes, con una velocidad de 1.969 GB/s.
// 								Entonces el bandwidth es: 1,969 GB/s * 16 lanes = 31,504 GB/s.
// -> DATOS ACUMULADOS: 32 GB/s * 820 ns ~= 26,240 KB.

// Datos por paquete: 64 bytes.
// -> SLOTS NECESARIOS: 26,240KB / 64B = aprox 410 slots.

const InputBufferCapacity = 512 // Por tener seguridad.

// La luz no espera, no se encola en la salida (debe ser 0).
// Akita requiere >0 para pasar el mensaje de Switch al cable.
const OutputBufferCapacity = 16 // Prevenimos deadlock.

func (s *Switch) CreatePort(name string) sim.Port {
	port := sim.NewPort(s, InputBufferCapacity, OutputBufferCapacity, name)
	s.connectedPorts = append(s.connectedPorts, port) // Pa la lista de connected ports.
	return port
}

// Configuración de la tabla de enrutamiento estática inicial.
func (s *Switch) RegisterDestination(dstName sim.RemotePort, outputPort sim.Port) {
	// TODO. El Controlador debe modificar dinámicamente esto.
	s.RouteTable[dstName] = outputPort
}

// --- LÓGICA PRINCIPAL DEL SWITCH ---
func (s *Switch) ProcessMsg(now sim.VTimeInSec, msg sim.Msg) {
	// TODO. Verificar en caso haya RECONFIGURACIÓN.
	// Podríamos descartar paquetes o ponerlos en buffers.

	dst := msg.Meta().Dst
	src := msg.Meta().Src

	// 1. Registrar Tráfico (para el predictor).
	s.RecordTraffic(now, src, dst, msg)

	// 2. Enrutar.
	outPort, found := s.RouteTable[dst]
	if !found { // Destino inexistente en la RouteTable.
		log.Panicf("[OPTICAL_SWITCH] Panic: No route from %s to %s", src, dst)
	}

	// 3. Verificando congestión de la salida.
	if !outPort.CanSend() {
		fmt.Printf("[OPTICAL_SWITCH] DROP! Port %s full sending to %s\n", outPort.Name(), dst)
		return
	}

	// 4. Reenviar (+ encapsulamiento).
	packet := &OpticalPacket{
		MsgMeta: sim.MsgMeta{
			ID:           sim.GetIDGenerator().Generate(), // Generamos ID válido
			Src:          outPort.AsRemote(),              // Switch firma como remitente.
			Dst:          dst,                             // Destino final se mantiene para el Link.
			TrafficBytes: 16,                              // ID del Paquete (8B) + Origen (4B) + Destino (4B).
			TrafficClass: "OpticalPacket",
		},
		InnerMsg: msg, // El mensaje es el original.
	}

	// Poniendo el packet en el buffer de salida -> el Port avisa al Link.
	outPort.Send(packet)
}

// --- EXTRACIÓN DE MÉTRICAS (tráfico) ---
func (s *Switch) RecordTraffic(now sim.VTimeInSec, src, dst sim.RemotePort, msg sim.Msg) {
	s.matrixLock.Lock()         // Tomamos el lock o esperamos.
	defer s.matrixLock.Unlock() // Devolvemos cuando termine la func.

	size := uint64(1)
	typeStr := "Unknown"

	// Para saber cuánto pesa cada mensaje, depende de su tipo.
	switch m := msg.(type) {
	case *mem.ReadReq:
		size = 8 // Arquitectura de 64 bits = 8 Bytes.
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

// --- REQUISITOS COMO COMPONENT: SWITCH ---
// Akita llama automáticamente cuando un Link deja algo en un inputPort.
func (s *Switch) NotifyRecv(port sim.Port) {
	now := s.Engine.CurrentTime()

	for { // Mientras hayan mensajes, procesa todos.
		req := port.RetrieveIncoming()
		if req == nil {
			return
		}

		s.ProcessMsg(now, req)
	}
}

func (s *Switch) NotifyPortFree(port sim.Port) {
	// No-op.
}

func (s *Switch) Handle(e sim.Event) error {
	return s.TickingComponent.Handle(e)
}
