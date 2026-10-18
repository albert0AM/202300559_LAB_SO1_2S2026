package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"strconv"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
)

type report struct {
	Country string `json:"country"`
	Planes  int    `json:"warplanes_in_air"`
	Ships   int    `json:"warships_in_water"`
	Time    string `json:"timestamp"`
}

var ctx = context.Background()

const luaMaxMin = `
local v = tonumber(ARGV[1])
local mx = redis.call('GET', KEYS[1])
if (not mx) or v > tonumber(mx) then redis.call('SET', KEYS[1], v) end
local mn = redis.call('GET', KEYS[2])
if (not mn) or v < tonumber(mn) then redis.call('SET', KEYS[2], v) end
return 1`

func store(rdb *redis.Client, r report) error {
	p := rdb.TxPipeline()
	p.Eval(ctx, luaMaxMin, []string{"planes:max", "planes:min"}, r.Planes)
	p.Eval(ctx, luaMaxMin, []string{"ships:max", "ships:min"}, r.Ships)
	p.HIncrBy(ctx, "freq:planes", strconv.Itoa(r.Planes), 1)
	p.HIncrBy(ctx, "freq:ships", strconv.Itoa(r.Ships), 1)
	p.ZIncrBy(ctx, "top:planes", float64(r.Planes), r.Country)
	p.ZIncrBy(ctx, "top:ships", float64(r.Ships), r.Country)
	p.HIncrBy(ctx, "reports:count", r.Country, 1)
	if r.Country == "GTM" {
		p.XAdd(ctx, &redis.XAddArgs{
			Stream: "gtm:series",
			MaxLen: 5000,
			Approx: true,
			Values: map[string]interface{}{"planes": r.Planes, "ships": r.Ships, "timestamp": r.Time},
		})
	}
	_, err := p.Exec(ctx)
	return err
}

func getenv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func main() {
	rdb := redis.NewClient(&redis.Options{Addr: getenv("VALKEY_ADDR", "valkey:6379")})
	var conn *amqp.Connection
	var err error
	for i := 0; i < 30; i++ {
		conn, err = amqp.Dial(getenv("RABBITMQ_URL", "amqp://guest:guest@rabbitmq:5672/"))
		if err == nil {
			break
		}
		log.Printf("esperando RabbitMQ: %v", err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		log.Fatal(err)
	}
	ch, err := conn.Channel()
	if err != nil {
		log.Fatal(err)
	}
	if _, err = ch.QueueDeclare("war_reports", true, false, false, false, nil); err != nil {
		log.Fatal(err)
	}
	if err = ch.Qos(50, 0, false); err != nil {
		log.Fatal(err)
	}
	msgs, err := ch.Consume("war_reports", "", false, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("consumer listo")
	for w := 0; w < 4; w++ {
		go func() {
			for d := range msgs {
				var r report
				if err := json.Unmarshal(d.Body, &r); err != nil {
					log.Printf("mensaje invalido: %v", err)
					d.Nack(false, false)
					continue
				}
				if err := store(rdb, r); err != nil {
					log.Printf("error guardando: %v", err)
					d.Nack(false, true)
					time.Sleep(time.Second)
					continue
				}
				d.Ack(false)
			}
		}()
	}
	select {}
}
