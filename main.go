package main

import (
	"bufio"
	_ "fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

type screenInfo struct {
	name string
	out  string
}

type allScreens map[string]screenInfo

func setup_fifo() *os.File {
	fifo_name := os.Getenv("FVWM3_STATUS_PIPE")

	if fifo_name == "" {
		log.Println("fifo defaulting to /tmp/fvwm3.pipe...")
		fifo_name = "/tmp/fvwm3.pipe"
	}

	pipe, err := os.Open(fifo_name)
	if err != nil {
		log.Fatal(err)
	}

	return pipe
}

func read_from_fifo(screen *allScreens, pipe *os.File, c chan string) {
	buf := bufio.NewReader(pipe)

	for {
		line, _, _ := buf.ReadLine()

		if string(line) == "" {
			return
		}
		c <- string(line)
	}
}

func getRandRScreens() allScreens {
	randr_cmd := exec.Command("xrandr", "--listactivemonitors")

	randr_out, err := randr_cmd.Output()
	if err != nil {
		log.Fatal(err)
	}

	pm := regexp.MustCompile(`\S+$`)

	lines := strings.Split(string(randr_out), "\n")

	if len(lines) == 0 {
		log.Fatal("Didn't get output from xrandr")
	}

	screens := make(allScreens)

	for _, line := range lines[1:] {
		names := pm.FindString(line)
		if names != "" {
			screens[names] = screenInfo{
				name: names,
			}

		}
	}

	return screens
}

func main() {

	c := make(chan string)

	pipe := setup_fifo()
	defer pipe.Close()

	// XRandR Stuff.
	sm := getRandRScreens()

	// Read from fifo.
	go read_from_fifo(&sm, pipe, c)
	for {
		select {
		case fromFifo := <-c:
			log.Println(fromFifo)
		}
	}
}
