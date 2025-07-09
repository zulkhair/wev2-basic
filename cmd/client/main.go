//nolint:forbidigo,errcheck,depguard,gocritic,gocognit // this is just an example
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"connectrpc.com/connect"
	"github.com/argus-labs/go-ecs/pkg/micro"
	gatewayv1 "github.com/argus-labs/go-ecs/proto/gen/go/gateway/v1"
	"github.com/argus-labs/go-ecs/proto/gen/go/gateway/v1/gatewayv1connect"
	iscv1 "github.com/argus-labs/go-ecs/proto/gen/go/isc/v1"
	microv1 "github.com/argus-labs/go-ecs/proto/gen/go/micro/v1"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

func main() {
	// Add flag for client ID
	clientID := flag.String("id", "default", "Unique identifier for this client")
	flag.Parse()

	if len(flag.Args()) < 1 {
		fmt.Println("Usage:")
		fmt.Println("  Create player: client create-player <nickname>")
		fmt.Println("  Attack player: client attack-player <target> <damage>")
		fmt.Println("  Debug log: client debug-log")
		fmt.Println("  Call external: client call-external <message>")
		fmt.Println("  Query: client query <json-query>")
		fmt.Println("  Listen: client listen <event-name> -id=<client-id>")
		fmt.Println("  Subscribe: client subscribe <event-name> -id=<client-id>")
		fmt.Println("  Unsubscribe: client unsubscribe <event-name> -id=<client-id>")
		fmt.Println("  Stream Epoch: client stream-epoch -id=<client-id>")
		os.Exit(1)
	}

	// Create client with the custom interceptor
	client := gatewayv1connect.NewShardServiceClient(
		http.DefaultClient,
		"http://localhost:8080",
		connect.WithInterceptors(&userAgentInterceptor{clientID: *clientID}),
	)

	// Parse command and arguments - adjust to use flag.Args() instead of os.Args
	cmdType := flag.Args()[0]
	cmdPayload, err := createMessagePayload(cmdType, flag.Args())
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Create service address
	serviceAddr := &microv1.ServiceAddress{
		Realm:        microv1.ServiceAddress_REALM_WORLD,
		Organization: "organization",
		Project:      "project",
		ServiceId:    "service",
	}

	ctx := context.Background()
	switch cmdType {
	case "query":
		handleQuery(ctx, client, serviceAddr, cmdPayload)
	case "listen":
		handleListen(ctx, client, serviceAddr, cmdPayload)
	case "subscribe":
		handleSubscribeEvents(ctx, client, serviceAddr, cmdPayload)
	case "unsubscribe":
		handleUnsubscribeEvents(ctx, client, serviceAddr, cmdPayload)
	case "stream-epoch":
		handleStreamEpoch(ctx, serviceAddr)
	default:
		handleCommand(ctx, client, serviceAddr, cmdType, cmdPayload)
	}
}

func handleCommand(
	ctx context.Context,
	client gatewayv1connect.ShardServiceClient,
	serviceAddr *microv1.ServiceAddress,
	cmdType string,
	payload map[string]any,
) {
	// Convert message payload to protobuf Struct
	pbStruct, err := structpb.NewStruct(payload)
	if err != nil {
		log.Fatalf("failed to create protobuf struct: %v", err)
	}

	// Send command request
	req := connect.NewRequest(&gatewayv1.SendCommandRequest{
		Address: serviceAddr,
		Command: &iscv1.Command{
			Name:    cmdType,
			Payload: pbStruct,
		},
	})
	_, err = client.SendCommand(ctx, req)
	if err != nil {
		log.Fatalf("failed to send command: %v", err)
	}
	fmt.Printf("Successfully sent %s command\n", cmdType)
}

func handleQuery(
	ctx context.Context,
	client gatewayv1connect.ShardServiceClient,
	serviceAddr *microv1.ServiceAddress,
	payload map[string]any,
) {
	// Convert []any to []string for Find field
	findArr := make([]string, len(payload["find"].([]any)))
	for i, v := range payload["find"].([]any) {
		findArr[i] = v.(string)
	}

	// Send query request
	var match iscv1.Query_Match
	switch payload["match"].(string) {
	case "exact":
		match = iscv1.Query_MATCH_EXACT
	case "contains":
		match = iscv1.Query_MATCH_CONTAINS
	default:
		match = iscv1.Query_MATCH_UNSPECIFIED
	}

	req := connect.NewRequest(&gatewayv1.QueryRequest{
		Address: serviceAddr,
		Query: &iscv1.Query{
			Find:  findArr,
			Match: match,
			Where: payload["where"].(string),
		},
	})
	resp, err := client.Query(ctx, req)
	if err != nil {
		log.Fatalf("failed to send query: %v", err)
	}

	// Get the results
	results := resp.Msg.GetResults().GetEntities()
	for _, result := range results {
		jsonBytes, err := protojson.Marshal(result)
		if err != nil {
			log.Fatalf("Failed to marshal QueryResult to JSON: %v", err)
		}
		fmt.Println(string(jsonBytes))
	}
}

func handleListen(
	ctx context.Context,
	client gatewayv1connect.ShardServiceClient,
	serviceAddr *microv1.ServiceAddress,
	payload map[string]any,
) {
	eventName := payload["event"].(string)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	fmt.Println("Listening for events... (Press Ctrl+C to exit)")
	req := connect.NewRequest(&gatewayv1.StartEventStreamRequest{
		Subscriptions: []*gatewayv1.EventSubscription{{
			Address: serviceAddr,
			Events:  []string{eventName},
		}},
	})
	stream, err := client.StartEventStream(ctx, req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to send subscribe request: %v\n", err)
		os.Exit(1)
	}

	// Discard the first event.
	_ = stream.Receive()

	for {
		ok := stream.Receive()
		if !ok {
			if err := stream.Err(); err != nil {
				fmt.Fprintf(os.Stderr, "stream error: %v\n", err)
				break
			}
			continue
		}
		event := stream.Msg()
		// Print event as JSON
		bz, _ := protojson.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(event)
		fmt.Printf("Received event: %s\n", string(bz))
	}
}

func handleSubscribeEvents(
	ctx context.Context,
	client gatewayv1connect.ShardServiceClient,
	serviceAddr *microv1.ServiceAddress,
	payload map[string]any,
) {
	req := connect.NewRequest(&gatewayv1.SubscribeEventsRequest{
		Subscriptions: []*gatewayv1.EventSubscription{{
			Address: serviceAddr,
			Events:  []string{payload["event"].(string)},
		}},
	})

	_, err := client.SubscribeEvents(ctx, req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to subscribe to event: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Successfully added event '%s' to stream\n", payload["event"])
}

func handleUnsubscribeEvents(
	ctx context.Context,
	client gatewayv1connect.ShardServiceClient,
	serviceAddr *microv1.ServiceAddress,
	payload map[string]any,
) {
	req := connect.NewRequest(&gatewayv1.UnsubscribeEventsRequest{
		Subscriptions: []*gatewayv1.EventSubscription{{
			Address: serviceAddr,
			Events:  []string{payload["event"].(string)},
		}},
	})

	_, err := client.UnsubscribeEvents(ctx, req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to unsubscribe from event: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Successfully removed event '%s' from stream\n", payload["event"])
}

func handleStreamEpoch(
	ctx context.Context,
	serviceAddr *microv1.ServiceAddress,
) {
	// Connect to NATS
	nc, err := nats.Connect("nats://localhost:4222", nats.UserInfo("nats", "nats"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect to NATS: %v\n", err)
		os.Exit(1)
	}
	defer nc.Close()

	// Create JetStream context
	js, err := jetstream.New(nc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create JetStream context: %v\n", err)
		os.Exit(1)
	}

	// Get the stream name and subject
	streamName := fmt.Sprintf("%s_%s_%s_epoch",
		serviceAddr.GetOrganization(), serviceAddr.GetProject(), serviceAddr.GetServiceId())
	subject := micro.Endpoint(serviceAddr, "epoch")

	// Create a consumer
	consumer, err := js.CreateOrUpdateConsumer(ctx, streamName, jetstream.ConsumerConfig{
		Name:          fmt.Sprintf("epoch-consumer-%d", time.Now().UnixNano()), // Use timestamp for unique consumer name
		FilterSubject: subject,
		DeliverPolicy: jetstream.DeliverAllPolicy,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create consumer: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Streaming epochs from %s... (Press Ctrl+C to exit)\n", subject)

	// Consume messages
	msgs, err := consumer.Consume(func(msg jetstream.Msg) {
		var epoch iscv1.Epoch
		if err := proto.Unmarshal(msg.Data(), &epoch); err != nil {
			fmt.Fprintf(os.Stderr, "failed to unmarshal epoch: %v\n", err)
			return
		}

		// Print epoch as JSON
		bz, err := protojson.MarshalOptions{Multiline: true, Indent: "  ", EmitDefaultValues: true}.Marshal(&epoch)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to marshal epoch to JSON: %v\n", err)
			return
		}
		fmt.Printf("Received epoch: %s\n", string(bz))

		// Acknowledge the message
		if err := msg.Ack(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to ack message: %v\n", err)
		}
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to consume messages: %v\n", err)
		os.Exit(1)
	}

	// Wait for context cancellation
	<-ctx.Done()
	msgs.Stop()
}

func createMessagePayload(cmdType string, args []string) (map[string]any, error) {
	switch cmdType {
	case "create-player":
		if len(args) != 2 {
			return nil, errors.New("usage: client create-player <nickname>")
		}
		return map[string]any{
			"nickname": args[1],
		}, nil

	case "attack-player":
		if len(args) != 3 {
			return nil, errors.New("usage: client attack-player <target> <damage>")
		}
		damage, err := strconv.ParseUint(args[2], 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid damage value: %w", err)
		}
		return map[string]any{
			"target": args[1],
			"damage": uint32(damage),
		}, nil

	case "call-external":
		if len(args) != 2 {
			return nil, errors.New("usage: client call-external <message>")
		}
		return map[string]any{
			"message": args[1],
		}, nil

	case "query":
		if len(args) != 2 {
			return nil, errors.New("usage: client query <json>")
		}

		var queryMap map[string]any
		if err := json.Unmarshal([]byte(args[1]), &queryMap); err != nil {
			return nil, fmt.Errorf("error: invalid JSON: %w", err)
		}
		return queryMap, nil

	case "listen":
		if len(args) != 2 {
			return nil, errors.New("usage: client listen <event-name>")
		}
		return map[string]any{
			"event": args[1],
		}, nil

	case "subscribe":
		if len(args) != 2 {
			return nil, errors.New("usage: client subscribe <event-name>")
		}
		return map[string]any{
			"event": args[1],
		}, nil

	case "unsubscribe":
		if len(args) != 2 {
			return nil, errors.New("usage: client unsubscribe <event-name>")
		}
		return map[string]any{
			"event": args[1],
		}, nil

	case "stream-epoch":
		if len(args) != 1 {
			return nil, errors.New("usage: client stream-epoch")
		}
		return map[string]any{}, nil

	default:
		return nil, fmt.Errorf("unknown command type: %s", cmdType)
	}
}

// Create a custom interceptor that adds User-Agent to all requests.
type userAgentInterceptor struct {
	clientID string
}

func (i *userAgentInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		req.Header().Set("User-Agent", fmt.Sprintf("client-%s", i.clientID))
		return next(ctx, req)
	}
}

func (i *userAgentInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
		conn := next(ctx, spec)
		conn.RequestHeader().Set("User-Agent", fmt.Sprintf("client-%s", i.clientID))
		return conn
	}
}

func (i *userAgentInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		return next(ctx, conn)
	}
}
