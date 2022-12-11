package main

import (
	"context"
	"fmt"
	"github.com/rueian/rueidis"
	"time"
)

func main() {
	client, err := rueidis.NewClient(rueidis.ClientOption{InitAddress: []string{"127.0.0.1:6379"}})
	if err != nil {
		panic(err)
	}
	defer client.Close()

	cmd := client.B()
	ctx := context.Background()

	// HSET myhash f v
	_ = client.Do(ctx, cmd.Hset().Key("myhash").FieldValue().FieldValue("f", "v").Build()).Error()
	// HGETALL myhash
	resp := client.DoCache(ctx, cmd.Hgetall().Key("myhash").Cache(), time.Minute)
	fmt.Println(resp.IsCacheHit()) // false
	fmt.Println(resp.AsStrMap())   // map[f:v]
	// cache hit on client side
	resp = client.DoCache(ctx, cmd.Hgetall().Key("myhash").Cache(), time.Minute)
	fmt.Println(resp.IsCacheHit()) // true
	fmt.Println(resp.AsStrMap())   // map[f:v]
}
