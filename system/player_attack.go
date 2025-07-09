package system

import (
	"wev2-basic/component"
	"wev2-basic/event"
	systemevent "wev2-basic/system_event"

	"github.com/argus-labs/go-ecs/pkg/cardinal"
)

type AttackPlayerCommand struct {
	cardinal.BaseCommand
	Target string
	Damage uint32
}

func (a AttackPlayerCommand) Name() string {
	return "attack-player"
}

type AttackPlayerSystemState struct {
	cardinal.BaseSystemState
	AttackPlayerCommands    cardinal.WithCommand[AttackPlayerCommand]
	PlayerDeathSystemEvents cardinal.WithSystemEventEmitter[systemevent.PlayerDeath]
	PlayerDeathEvents       cardinal.WithEvent[event.PlayerDeath]
	Players                 PlayerSearch
}

func AttackPlayerSystem(state *AttackPlayerSystemState) error {
	for msg := range state.AttackPlayerCommands.Iter() {
		for entity, player := range state.Players.Iter() {
			tag := player.Tag.Get()

			if msg.Target != tag.Nickname {
				continue
			}

			newHealth := player.Health.Get().HP - int(msg.Damage)
			if newHealth > 0 {
				player.Health.Set(component.Health{HP: newHealth})

				state.Logger().Info().
					Uint32("entity", uint32(entity.ID)).
					Msgf("Player %s received %d damage", msg.Target, msg.Damage)
			} else {
				entity.Destroy()

				state.PlayerDeathEvents.Emit(event.PlayerDeath{Nickname: tag.Nickname})

				state.PlayerDeathSystemEvents.Emit(systemevent.PlayerDeath{Nickname: tag.Nickname})

				state.Logger().Info().Uint32("entity", uint32(entity.ID)).Msgf("Player %s died", msg.Target)
			}
		}
	}
	return nil
}
