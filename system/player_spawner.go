package system

import (
	"wev2-basic/component"
	"wev2-basic/event"

	"github.com/argus-labs/go-ecs/pkg/cardinal"
)

type CreatePlayerCommand struct {
	cardinal.BaseCommand
	Nickname string `json:"nickname"`
}

func (a CreatePlayerCommand) Name() string {
	return "create-player"
}

type CreatePlayerSystemState struct {
	cardinal.BaseSystemState
	CreatePlayerCommands cardinal.WithCommand[CreatePlayerCommand]
	NewPlayerEvents      cardinal.WithEvent[event.NewPlayer]
	Players              PlayerSearch
}

func CreatePlayerSystem(state *CreatePlayerSystemState) error {
	for msg := range state.CreatePlayerCommands.Iter() {
		id, err := state.Players.Create(
			component.PlayerTag{Nickname: msg.Nickname},
			component.Health{HP: 100},
		)
		if err != nil {
			// If we return the error, Cardinal will shutdown, so just log it.
			state.Logger().Error().Err(err).Msg("error creating entity")
			continue
		}

		state.NewPlayerEvents.Emit(event.NewPlayer{Nickname: msg.Nickname})
		state.Logger().Info().Uint32("entity", uint32(id)).Msgf("Created player %s", msg.Nickname)
	}
	return nil
}
