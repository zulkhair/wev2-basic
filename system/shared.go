package system

import (
	"wev2-basic/component"

	"github.com/argus-labs/go-ecs/pkg/ecs"
)

type PlayerSearch = ecs.Exact[struct {
	Tag    ecs.Ref[component.PlayerTag]
	Health ecs.Ref[component.Health]
}]

type GraveSearch = ecs.Exact[struct {
	Grave ecs.Ref[component.Gravestone]
}]
