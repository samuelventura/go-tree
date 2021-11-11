package tree

import (
	"fmt"
	"testing"
	"time"
)

type tlog struct {
	out chan string
}

func (tl *tlog) pln(args ...interface{}) {
	tl.out <- fmt.Sprintln(args...)
}

func (tl *tlog) wait_and_print(c <-chan interface{}, args ...interface{}) {
	<-c
	tl.out <- fmt.Sprintln(args...)
}

func (tl *tlog) wait_to_equal(t *testing.T, millis int, pattern string) {
	tl.wait_to(t, millis, func(line string) bool {
		return line == pattern
	})
}

func (tl *tlog) wait_to(t *testing.T, millis int, checker func(string) bool) {
	done := make(chan interface{})
	go func() {
		for {
			timer := time.NewTimer(time.Duration(millis) * time.Millisecond)
			select {
			case <-timer.C:
				t.Error("timeout")
				done <- true
				return
			case line := <-tl.out:
				timer.Stop()
				fmt.Print(line)
				if checker(line) {
					done <- true
					return
				}
			}
		}
	}()
	<-done
}
