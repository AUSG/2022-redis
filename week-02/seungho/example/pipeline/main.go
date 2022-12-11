package main

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"strconv"
)

func main() {
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	ctx := context.Background()
	pipe := client.Pipeline()
	for i := 0; i < 10; i++ {
		pipe.Set(ctx, "testkey:"+strconv.Itoa(i), strconv.Itoa(i), 0)
	}
	_, err := pipe.Exec(ctx)
	if err != nil {
		panic(err)
	}

	cmds := map[string]*redis.StringCmd{}
	for i := 0; i < 10; i++ {
		key := "testkey:" + strconv.Itoa(i)
		cmds[key] = pipe.Get(ctx, key)
	}
	for i := 0; i < 10; i++ {
		key := "testkey:" + strconv.Itoa(i)
		err := pipe.Del(ctx, key).Err()
		if err != nil {
			panic(err)
		}
	}
	_, err = pipe.Exec(ctx)
	if err != nil {
		panic(err)
	}
	for k, v := range cmds {
		val, err := v.Result()
		if err != nil {
			panic(err)
		}
		fmt.Printf("    %s  %s\n", k, val)
	}
}
