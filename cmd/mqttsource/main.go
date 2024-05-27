/*
Copyright 2024 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"flag"
	"log"
	"net"
	"os"

	mqtt_paho "github.com/cloudevents/sdk-go/protocol/mqtt_paho/v2"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/eclipse/paho.golang/paho"
)

var (
	sink   string
	source string

	// CloudEvents specific parameters
	eventType   string
	eventSource string

	topic    string
	clientid string
)

func init() {
	flag.StringVar(&sink, "sink", "", "the host url to send messages to")
	flag.StringVar(&source, "source", "", "the url to get messages from")
	flag.StringVar(&eventType, "eventType", "mqtt-event", "the event-type (CloudEvents)")
	flag.StringVar(&eventSource, "eventSource", "", "the event-source (CloudEvents)")

	flag.StringVar(&topic, "topic", "mqtt-topic", "MQTT topic subscribe to")
	flag.StringVar(&clientid, "clientid", "receiver-client-id", "MQTT source client id")
}

func main() {
	flag.Parse()

	k_sink := os.Getenv("K_SINK")
	if k_sink != "" {
		sink = k_sink
	}

	// "source" flag must not be empty for operation.
	if source == "" {
		log.Fatal("A valid MQTT broker URL must be defined.")
	}

	// The event's source defaults to the MQTT broker URL.
	if eventSource == "" {
		eventSource = source
	}

	ctx := cloudevents.ContextWithTarget(context.Background(), sink)

	conn, err := net.Dial("tcp", source)
	if err != nil {
		log.Fatalf("failed to connect to MQTT broker: %s", err.Error())
	}

	config := &paho.ClientConfig{
		ClientID: clientid,
		Conn:     conn,
	}

	subscribeOpt := &paho.Subscribe{
		Subscriptions: []paho.SubscribeOptions{
			{Topic: topic,
				QoS: 0},
		},
	}

	p_receive, err := mqtt_paho.New(ctx, config, mqtt_paho.WithSubscribe(subscribeOpt))
	if err != nil {
		log.Fatalf("failed to create protocol: %s", err.Error())
	}
	defer p_receive.Close(ctx)

	c_receive, err := cloudevents.NewClient(p_receive)
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
	}

	c_send, err := cloudevents.NewClientHTTP()
	if err != nil {
		log.Fatalf("Failed to create a http cloudevent client: %s", err.Error())
	}

	log.Printf("MQTT source start consuming messages from %s\n", source)
	err = c_receive.StartReceiver(ctx, func(ctx context.Context, event cloudevents.Event) {
		receive(ctx, event, c_send)
	})
	if err != nil {
		log.Fatalf("failed to start receiver: %s", err)
	} else {
		log.Printf("MQTT source stopped\n")
	}

}

func receive(ctx context.Context, event cloudevents.Event, c cloudevents.Client) {
	log.Printf("Received event: %s", event)
	data := event.Data()
	newEvent := cloudevents.NewEvent(cloudevents.VersionV1)
	newEvent.SetType(eventType)
	newEvent.SetSource(eventSource)
	newEvent.SetID(event.ID())
	_ = newEvent.SetData(cloudevents.ApplicationJSON, data)
	if result := c.Send(ctx, newEvent); !cloudevents.IsACK(result) {
		log.Printf("Sending event to sink failed: %v", result)
	}
}
