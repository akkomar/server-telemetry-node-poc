package main

import (
    "log"
    "time"
)

func main() {
    for {
        log.Println("Logging in a loop...")
        time.Sleep(time.Second)
    }
}
