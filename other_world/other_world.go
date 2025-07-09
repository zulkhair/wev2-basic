package otherworld

import "github.com/argus-labs/go-ecs/pkg/cardinal"

// Matchmaking is another shard. Just for example send this to itself.
var Matchmaking = cardinal.OtherWorld{ //nolint:gochecknoglobals // it's fine
	Organization: "organization",
	Project:      "project",
	ServiceID:    "service",
}
