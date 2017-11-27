package main

import (
	"log"
    "strconv"
    "fmt"
	"github.com/streadway/amqp"
	"encoding/json"
	"zvelo.io/ttlru"
	"time"
	"mqBuilder"
)

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}

type Request struct {
    Operation string      `json:"operation"`
    Key string 			  `json:"key"`
    Value string		  `json:"value"`
}

func main() {
	conn, ch := mqBuilder.ConnectMQ()
	defer conn.Close()
	defer ch.Close()

	q := mqBuilder.DeclareServerQueue(ch, "rpc_queue")

	msgs := mqBuilder.ConsumeQueue(ch, q.Name)

	forever := make(chan bool)

	cache := ttlru.New(100, ttlru.WithTTL(5 * time.Minute))

	for d := range msgs {    
    	go hitCache(cache, d)
    }	

	log.Printf(" [*] Waiting for messages. To exit press CTRL+C")
	<-forever
}


func hitCache(cache ttlru.Cache, d amqp.Delivery) {
	
	log.Printf("Received a message: %s", d.Body)
	data := Request{}
	err := json.Unmarshal(d.Body, &data)
	failOnError(err, "Failed to read json")

	switch data.Operation {
    	case "get":
    		_, ch := mqBuilder.ConnectMQ()

       		value, succ := cache.Get(data.Key)
			log.Printf(strconv.FormatBool(succ))
			log.Printf(value.(string))
			mqBuilder.PublishQueue(ch, d.ReplyTo, "", d.CorrelationId, value.(string))
    	case "set", "update":
       		succ := cache.Set(data.Key, data.Value)
       		log.Printf(data.Key)
			log.Printf(strconv.FormatBool(succ))
       		log.Printf(strconv.Itoa(cache.Len()))
       	case "remove":
       		succ := cache.Del(data.Key)
       		log.Printf(strconv.FormatBool(succ))
       	case "keys":
       		keys := cache.Keys()
       		string_keys := make([]string, len(keys))
			for i, v := range keys {
    			string_keys[i] = fmt.Sprint(v)
			}
       		_, ch := mqBuilder.ConnectMQ()

			failOnError(err, "Failed to declare a queue")

			mqBuilder.PublishQueue(ch, d.ReplyTo, "", d.CorrelationId, fmt.Sprintf("%#v\n", string_keys))
       	default:
       		log.Printf("Unknown operation: %s", data.Operation)	
   	} 	
}