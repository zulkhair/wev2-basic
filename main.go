package main

import (
	"wev2-basic/system"

	"github.com/argus-labs/go-ecs/pkg/cardinal"
)

func main() {
	world := cardinal.NewWorld()

	cardinal.RegisterSystem(world, system.PlayerSpawnerSystem, cardinal.WithHook(cardinal.Init))

	cardinal.RegisterSystem(world, system.CreatePlayerSystem)
	cardinal.RegisterSystem(world, system.RegenSystem)
	cardinal.RegisterSystem(world, system.AttackPlayerSystem)
	cardinal.RegisterSystem(world, system.GraveyardSystem)
	cardinal.RegisterSystem(world, system.CallExternalSystem)

	world.StartGame()
}
