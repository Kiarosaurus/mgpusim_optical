package optical

import (
	"fmt"

	"github.com/sarchlab/akita/v4/sim"
	"github.com/sarchlab/akita/v4/simulation"
)

// Conecta puertos de GPU al Switch.
type Connector struct {
	Simulation *simulation.Simulation
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

// Conecta un puerto de GPU/Memoria al Switch Óptico.
func (c *Connector) PlugIn(gpuPort sim.Port) {
	// 1. Crear puerto en el Switch.
	switchPortName := fmt.Sprintf("Port[%d]", c.portID)
	c.portID++
	switchPort := c.Switch.CreatePort(switchPortName)

	// 2. Crear el Link (fibra óptica).
	cableName := fmt.Sprintf("Fiber[%d]", c.portID)
	cable := NewLink(cableName, c.Simulation.GetEngine(), 1e-9)

	// 3. Registrar el Link en la simulación (para que procese eventos).
	c.Simulation.RegisterComponent(cable)

	// 4. Conectando físicamente.
	cable.PlugIn(gpuPort)    // Link con GPU.
	cable.PlugIn(switchPort) // Link con Switch.

	// 5. Ruta... (por ahora, estática).
	// TODO. Hacerlo para topología reconfigurable.
	c.Switch.RegisterDestination(gpuPort.AsRemote(), switchPort)
}
