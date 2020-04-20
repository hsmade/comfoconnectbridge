package main

import (
	"log"
	"time"

	"gopkg.in/tomb.v2"
)
func main() {
	s := s1{}
	s.t.Go(s.Run)

	time.Sleep(1 * time.Second)

	log.Print("kill")
	s.t.Kill(nil)

	log.Print("wait")
	s.t.Wait()

	log.Print("all dead")
}

type s1 struct {
	t tomb.Tomb
}

func (s *s1) Run() error {
	s.t.Go(s.loop)
	t := time.NewTicker(500 * time.Millisecond)
	for {
		select {
		case <- t.C:
			log.Print("s1:.")
		case 	<- s.t.Dying():
			log.Print("s1: died")
			return nil
		}
	}
}

func (s *s1) loop() error {
	t := time.NewTicker(500 * time.Millisecond)
	for {
		select {
		case <-t.C:
			log.Print("loop:.")
		case 	<- s.t.Dying():
			log.Print("loop: died")
			return nil

		}
	}
}