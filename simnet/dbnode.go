package simnet

import (
	"log"
	"os"
	"strconv"
)

func startNode(id int, incoming chan message, outgoing chan message) {
	for msg := range incoming {
		if msg.dest != id {
			log.Print("Misdelivered message for %d delivered to %d", msg.dest, id)
		} else {
			switch msg.demuxKey {
			case clientReadRequest:
				//				f, err := os.Open("x" + strconv.Itoa(len(msg.value)) + ".dat")
				//				if err != nil {
				//					panic(err)
				//				}
				//
				//				f.Read(msg.value)
				//				f.Close()

				outgoing <- message{
					msg.id,
					id,
					msg.src,
					clientReadResponse,
					msg.key,
					make([]byte, 0),
					false,
				}
			case clientWriteRequest:
				f, err := os.Open("x" + strconv.Itoa(len(msg.value)) + ".dat")
				if err != nil {
					panic(err)
				}

				f.Read(msg.value)
				f.Close()

				f, err = os.Create(strconv.Itoa(msg.id) + ".dat")
				if err != nil {
					panic(err)
				}

				f.Write(msg.value)
				f.Close()

				os.Remove(strconv.Itoa(msg.id) + ".dat")

				outgoing <- message{
					msg.id,
					id,
					msg.src,
					clientWriteResponse,
					msg.key,
					msg.value,
					false,
				}
			default:
				log.Print("Unexpected message type received %+v", msg)
			}
		}
	}
}
