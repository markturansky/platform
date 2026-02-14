package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ambient-code/platform/components/ambient-sdk/go-sdk/client"
	"github.com/ambient-code/platform/components/ambient-sdk/go-sdk/types"
)

func main() {
	fmt.Println("Ambient Platform SDK - Go Example")
	fmt.Println("==================================")

	c, err := client.NewClientFromEnv(client.WithTimeout(60 * time.Second))
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	ctx := context.Background()

	session, err := types.NewSessionBuilder().
		Name("example-session").
		Prompt("Analyze the repository structure").
		Build()
	if err != nil {
		log.Fatalf("Failed to build session: %v", err)
	}

	created, err := c.Sessions().Create(ctx, session)
	if err != nil {
		log.Fatalf("Failed to create session: %v", err)
	}
	fmt.Printf("Created session: %s (id=%s)\n", created.Name, created.ID)

	got, err := c.Sessions().Get(ctx, created.ID)
	if err != nil {
		log.Fatalf("Failed to get session: %v", err)
	}
	fmt.Printf("Got session: %s\n", got.Name)

	opts := types.NewListOptions().Size(10).Build()
	list, err := c.Sessions().List(ctx, opts)
	if err != nil {
		log.Fatalf("Failed to list sessions: %v", err)
	}
	fmt.Printf("Found %d sessions (total: %d)\n", len(list.Items), list.Total)

	patch := types.NewSessionPatchBuilder().
		Prompt("Updated prompt").
		Build()
	updated, err := c.Sessions().Update(ctx, created.ID, patch)
	if err != nil {
		log.Fatalf("Failed to update session: %v", err)
	}
	fmt.Printf("Updated session prompt: %s\n", updated.Prompt)

	fmt.Println("\nIterating all sessions:")
	iter := c.Sessions().ListAll(ctx, types.NewListOptions().Size(100).Build())
	count := 0
	for iter.Next() {
		s := iter.Item()
		count++
		if count <= 3 {
			fmt.Printf("  %d. %s (%s)\n", count, s.Name, s.ID)
		}
	}
	if err := iter.Err(); err != nil {
		log.Fatalf("Iteration error: %v", err)
	}
	if count > 3 {
		fmt.Printf("  ... and %d more\n", count-3)
	}

	fmt.Println("\nDone.")
}
