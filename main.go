package main

import (
	"bufio"
	_ "cmp"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
)

type screenInfo struct {
	Name string
	Out  string
}

type jsonBlobDesktops struct {
	Number       int  `json:"number"`
	IsCurrent    bool `json:"is_current"`
	IsUrgent     bool `json:"is_urgent,omitempty"`
	NumOfClients int  `json:"number_of_clients"`
}

type jsonBlobScreens struct {
	Desktops      map[string]jsonBlobDesktops `json:"desktops"`
	CurrentClient string                      `json:"current_client,omitempty"`
	RandrOrder    int                         `json:"randr_order"`
}

type jsonBlob struct {
	Version       int64                      `json:"version"`
	Currentscreen string                     `json:"current_screen"`
	DesktopMode   string                     `json:"desktop_mode"`
	Screens       map[string]jsonBlobScreens `json:"screens"`
}

type allScreens map[string]*screenInfo

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

func read_from_fifo(screenmap *allScreens, pipe *os.File, c chan string) {
	buf := bufio.NewReader(pipe)

	go func() {
		var clockLine string
		for {
			var data jsonBlob

			line, _ := buf.ReadString('\n')

			if line == "" {
				continue
			}
			if strings.HasPrefix(line, "clock:") {
				clockLine = strings.TrimPrefix(line, "clock:")
			} else {
				err := json.Unmarshal([]byte(line), &data)
				if err != nil {
					continue
				}
			}

			// Process the JSON data here.
			for screen, _ := range data.Screens {
				var msg strings.Builder
				var extra_msg strings.Builder
				var extra_urgent strings.Builder

				fmt.Fprintf(&msg, "%%{Sn%s}", screen)

				var scr_h = data.Screens[screen]
				var sorted_scr_h []struct {
					Key   string
					Value jsonBlobDesktops
				}

				for k, v := range scr_h.Desktops {
					sorted_scr_h = append(sorted_scr_h, struct {
						Key   string
						Value jsonBlobDesktops
					}{k, v})
				}

				sort.Slice(sorted_scr_h, func(i, j int) bool {
					return sorted_scr_h[i].Value.Number < sorted_scr_h[j].Value.Number
				})

				for _, deskinfo := range sorted_scr_h {
					deskname := deskinfo.Key
					if deskinfo.Value.IsCurrent {
						fmt.Fprintf(&msg, "|%%{B#39c488} %s %%{B-}", deskname)
						fmt.Fprintf(&extra_msg,
							"%%{B#D7C72F}[Scr:%s][N:%d][A:%d][L:%s]%%{B-}",
							screen,
							scr_h.RandrOrder,
							deskinfo.Value.NumOfClients,
							data.DesktopMode)
					} else {
						if deskinfo.Value.NumOfClients > 0 {
							fmt.Fprintf(&msg, "|%%{B#004C98} %s %%{B-}", deskname)
						} else if deskinfo.Value.NumOfClients == 0 {
							continue
						}
					}
				}

				fmt.Fprintf(&msg, "%%{F#FF00FF}|%%{F-}%s", extra_msg.String())

				if scr_h.CurrentClient != "" {
					var cc = scr_h.CurrentClient
					fmt.Fprintf(&msg,
						"%%{c}%%{U#00FF00}%%{+u}%%{+o}%%{B#AC59FF}%%{F-}        %s        %%{-u}%%{-o}%%{B-}", cc)
				}
				_ = extra_urgent

				if (*screenmap)[screen] == nil {
					(*screenmap)[screen] = &screenInfo{
						Name: screen,
						Out:  "",
					}
				}
				(*screenmap)[screen].Out = msg.String()
				msg.Reset()
				extra_msg.Reset()
			}

			for _, v := range *screenmap {
				c <- fmt.Sprintf("%s%s", v.Out, clockLine)
			}
		}
	}()
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
			screens[names] = &screenInfo{
				Name: names,
				Out:  "",
			}

		}
	}

	return screens
}

func main() {

	c := make(chan string)

	pipe := setup_fifo()
	defer pipe.Close()

	if os.Getenv("FRS_PROFILE") == "1" {
		go func() {
			log.Println(http.ListenAndServe("localhost:6060", nil))
		}()
	}

	// XRandR Stuff.
	sm := getRandRScreens()

	// Read from fifo.
	go read_from_fifo(&sm, pipe, c)
	for {
		select {
		case fromFifo := <-c:
			fmt.Println(fromFifo)
		}
	}
}
