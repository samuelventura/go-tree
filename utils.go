package tree

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

type tlog struct {
	out chan string
}

func (tl *tlog) pln(args ...interface{}) {
	tl.out <- fmt.Sprintln(args...)
}

func (tl *tlog) ftl(args ...interface{}) {
	tl.out <- fmt.Sprintln(args...)
	os.Exit(1)
}

func (tl *tlog) w(c <-chan interface{}, args ...interface{}) {
	<-c
	tl.out <- fmt.Sprintln(args...)
}

func (tl *tlog) tos(t *testing.T, millis int, pattern string) {
	tl.to(t, millis, func(line string) bool {
		return strings.Contains(line, pattern)
	})
}

func (tl *tlog) tose(t *testing.T, millis int, pattern string) {
	tl.to(t, millis, func(line string) bool {
		return line == pattern
	})
}

func (tl *tlog) to(t *testing.T, millis int, checker func(string) bool) {
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
