package optical

import (
	"fmt"

	"github.com/sarchlab/akita/v4/sim"
	"github.com/sarchlab/akita/v4/simulation"
)

// Conecta puertos de GPU al Switch.
type Connector struct {
	Simulation *simulation.Simulation // Ref a la simulación completa.
	Switch     *Switch
	portID     int
}

func NewConnector(sim *simulation.Simulation) *Connector {
	// Switch centralizada.
	sw := NewSwitch("OpticalSwitch", sim.GetEngine())

	return &Connector{
		Simulation: sim,
		Switch:     sw,
		portID:     0,
	}
}

// Conecta un Port de GPU/Memoria al Switch Óptico.
func (c *Connector) PlugIn(gpuPort sim.Port) {
	// 1. Crear Port genérico en el Switch.
	switchPortName := fmt.Sprintf("Port[%d]", c.portID)
	c.portID++
	switchPort := c.Switch.CreatePort(switchPortName)

	// 2. Crear el Link (fibra óptica).
	cableName := fmt.Sprintf("Fiber[%d]", c.portID)

	// Velocidad de la luz en el vacío (c) = 3 x 10^8 m/s aprox.
	// Índice de refracción del silicio (n) = 1,5 aprox.
	// Vel. de la luz en fibra = c/n = 2 x 10^8.
	//
	// Vamos a considerar que el GPU y el Switch Óptico se encuentran en un mismo rack.
	// En ese caso, la distancia del cable que los conecta (d) es entre 1-3 metros. Elegimos d=2.
	// d = v x t, es decir t = d/v.
	// t = 20 m / 2 x 10^8 m/s  =>  1 x 10^{-8}
	cable := NewLink(cableName, c.Simulation.GetEngine(), 1e-8)

	// 3. Registrar el Link en la simulación (para que procese eventos).
	c.Simulation.RegisterComponent(cable)

	// 4. Conectando físicamente.
	cable.PlugIn(gpuPort)    // Link con GPU.
	cable.PlugIn(switchPort) // Link con Switch.

	// 5. Ruta... (por ahora, estática).
	// TODO. Hacerlo para topología reconfigurable.
	c.Switch.RegisterDestination(gpuPort.AsRemote(), switchPort)
}
